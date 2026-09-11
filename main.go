package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"
	"time"

	i "github.com/SinTan1729/self-ip/internal"
)

var Version = "unknown"

func basicHandler(w http.ResponseWriter, r *http.Request, appData *i.AppData) {
	url, err := url.Parse(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}
	clientIP, queryIP := i.GetClientIP(url, r, appData.Proxies)
	var parsedQueryIP netip.Addr
	badIP := false
	if ip, err := netip.ParseAddr(queryIP); err != nil {
		badIP = true
	} else {
		parsedQueryIP = ip
	}

	var mode i.Mode
	switch url.Query().Get("mode") {
	case "ip_only":
		mode = i.IPOnly
	case "full":
		mode = i.Full
	case "echoip":
		mode = i.EchoIP
	case "short":
		mode = i.Short
	case "", "default":
		mode = i.Default
	default:
		log.Println(i.LogText(clientIP, mode, queryIP, i.BadAttempt))
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}
	if badIP {
		log.Println(i.LogText(clientIP, mode, queryIP, i.BadAttempt))
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}

	if !i.CheckAuth(appData.ApiKey, r.Header.Get("X-API-Key")) {
		log.Println(i.LogText(clientIP, mode, queryIP, i.Unauthorized))
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
		http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
		return
	}

	data := appData.Databases.GetGeoData(parsedQueryIP, mode, r.UserAgent())
	if data == nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	if mode != i.IPOnly {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/plain")
	}

	log.Println(i.LogText(clientIP, mode, queryIP, i.GoodAttempt))
	fmt.Fprintf(w, "%s", data)
}

func portHandler(w http.ResponseWriter, r *http.Request, appData *i.AppData) {
	mode := i.PortCheck
	url, err := url.Parse(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}
	clientIP, queryIP := i.GetClientIP(url, r, appData.Proxies)

	var badRequest string
	port := 443 // Default to https port
	providedPort := r.URL.Query().Get("port")
	if p, err := strconv.Atoi(providedPort); err == nil && p > 0 && p < 65536 {
		port = p
	} else if providedPort != "" {
		badRequest = "Bad port"
	}

	var ip netip.Addr
	if tIP, err := netip.ParseAddr(queryIP); err == nil {
		ip = tIP
	} else {
		badRequest = "Bad Provided IP"
	}
	if !ip.IsGlobalUnicast() || slices.Contains(appData.OwnIP, ip) || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		log.Println("Blocked IP was requested:", ip)
		badRequest = "Blocked IP"
	}

	address := fmt.Sprintf("[%s]:%d", queryIP, port)
	logAddress := address
	if queryIP == clientIP {
		logAddress = fmt.Sprintf(":%d", port)
	}

	if !i.CheckAuth(appData.ApiKey, r.Header.Get("X-API-Key")) {
		log.Println(i.LogText(clientIP, mode, logAddress, i.Unauthorized))
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
		http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
		return
	}
	if badRequest != "" {
		log.Println(i.LogText(clientIP, mode, queryIP, i.BadAttempt))
		http.Error(w, "400 Bad Request: "+badRequest, http.StatusBadRequest)
		return
	}

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)

	var res i.PortResponse
	res.IP = ip
	res.Port = uint16(port)

	if err != nil {
		switch {
		case os.IsTimeout(err):
			res.Status = i.PortTimeout
		case errors.Is(err, syscall.ECONNREFUSED):
			res.Status = i.PortRefused
		case errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH):
			res.Status = i.PortUnreachable
		default:
			res.Status = i.PortUnknown
		}
	} else {
		res.Status = i.PortOpen
		defer conn.Close()
	}
	res.Reachable = res.Status == i.PortOpen

	jsonData, err := json.Marshal(res)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(i.LogText(clientIP, mode, logAddress, i.GoodAttempt))
	fmt.Fprintf(w, string(jsonData))
}

func healthHandler(w http.ResponseWriter, r *http.Request, appData *i.AppData) {
	if appData.Databases.Healthy() {
		fmt.Fprintf(w, "healthy")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unhealthy")
	}
}

func main() {
	log.SetFlags(0)
	log.SetOutput(new(i.LogWriter))

	if Version == "unknown" {
		log.Println(i.Blue + "Self IP (dev build)" + i.Reset)
	} else {
		log.Printf(i.Blue+"Self IP v%s"+i.Reset, Version)
	}
	log.Println(i.Blue + "https://github.com/SinTan1729/self-ip" + i.Reset)
	log.Println("-----------------")

	i.GetDatabases()

	databases := &i.DatabaseStore{}
	if err := databases.Reload(); err != nil {
		log.Fatal(err)
	}
	defer databases.Close()

	apiKey, flag := os.LookupEnv("SELF_IP_API_KEY")
	if !flag {
		log.Fatal("No API key was provided.")
	}

	var trustedProxies []netip.Prefix
	go databases.ScheduleUpdates()
	if trustedProxiesEnv, flag := os.LookupEnv("SELF_IP_TRUSTED_PROXIES"); flag {
		if p, err := i.ParseTrustedProxies(trustedProxiesEnv); err == nil {
			trustedProxies = p
		} else {
			log.Fatal("Error processing trusted proxies:", err)
		}
	}
	if trustedProxies != nil {
		log.Println("Using trusted proxies:", i.PrettyPrintArray(trustedProxies))
	}

	appData := i.AppData{
		Databases: databases,
		ApiKey:    apiKey,
		Proxies:   trustedProxies,
	}
	if ownIPs := i.GetOwnIPs(); ownIPs != nil {
		log.Println("Resolved own IP(s):", i.PrettyPrintArray(ownIPs))
		appData.OwnIP = ownIPs
	}

	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/json", "/api":
			basicHandler(w, r, &appData)
		case "/portcheck":
			portHandler(w, r, &appData)
		default:
			http.Error(w, "404 Page Not Found", http.StatusNotFound)
		}
	})
	public := &http.Server{
		Addr:    ":3213",
		Handler: publicMux,
	}

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		healthHandler(w, r, &appData)
	})
	health := &http.Server{
		Addr:    "127.0.0.1:1729",
		Handler: healthMux,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := health.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Printf("health: %v", err)
		}
	}()

	go func() {
		if err := public.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Printf("public: %v", err)
		}
	}()

	log.Println("Server running at http://localhost:3213")
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	public.Shutdown(ctx)
	health.Shutdown(ctx)
}

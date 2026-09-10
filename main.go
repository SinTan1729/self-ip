package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"syscall"
	"time"

	i "github.com/SinTan1729/self-ip/internal"
)

var Version = "unknown"

func basicHandler(w http.ResponseWriter, r *http.Request, databases *i.DatabaseStore, apiKey string, trustedProxies []netip.Prefix) {
	url, err := url.Parse(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}
	clientIP, queryIP := i.GetClientIP(url, r, trustedProxies)

	var mode i.Mode
	switch url.Query().Get("mode") {
	case "ip_only":
		mode = i.IPOnly
	case "full":
		mode = i.Full
	case "echoip":
		mode = i.EchoIP
	case "", "default":
		mode = i.Default
	default:
		log.Println(i.LogText(clientIP, mode, queryIP, i.BadAttempt))
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}

	if !i.CheckAuth(apiKey, r.Header.Get("X-API-Key")) {
		log.Println(i.LogText(clientIP, mode, queryIP, i.Unauthorized))
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
		http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
		return
	}

	data := databases.GetGeoData(queryIP, mode, r.UserAgent())
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

func portHandler(w http.ResponseWriter, r *http.Request, apiKey string, trustedProxies []netip.Prefix) {
	mode := i.PortCheck
	url, err := url.Parse(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}
	clientIP, queryIP := i.GetClientIP(url, r, trustedProxies)

	badRequest := false
	port := 443 // Default to https port
	providedPort := r.URL.Query().Get("port")
	if p, err := strconv.Atoi(providedPort); err == nil && p > 0 && p < 65536 {
		port = p
	} else if providedPort != "" {
		badRequest = true
	}

	ip := netip.MustParseAddr(queryIP).Unmap()
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		log.Println("Blocked IP was requested:", ip)
		badRequest = true
	}

	address := fmt.Sprintf("[%s]:%d", queryIP, port)
	logAddress := address
	if queryIP == clientIP {
		logAddress = fmt.Sprintf(":%d", port)
	}

	if !i.CheckAuth(apiKey, r.Header.Get("X-API-Key")) {
		log.Println(i.LogText(clientIP, mode, logAddress, i.Unauthorized))
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
		http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
		return
	}
	if badRequest {
		log.Println(i.LogText(clientIP, mode, queryIP, i.BadAttempt))
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)

	var res i.PortResponse
	res.IP = queryIP
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
	log.Println("Using trusted proxies:", trustedProxies)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/json", "/api":
			basicHandler(w, r, databases, apiKey, trustedProxies)
		case "/portcheck":
			portHandler(w, r, apiKey, trustedProxies)
		default:
			http.Error(w, "404 Page Not Found", http.StatusNotFound)
		}
	})

	log.Println("Server running at http://localhost:3213")
	if err := http.ListenAndServe(":3213", nil); err != nil {
		panic(err)
	}
}

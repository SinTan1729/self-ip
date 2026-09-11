package handlers

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
	"slices"
	"strconv"
	"syscall"
	"time"

	i "github.com/SinTan1729/self-ip/internal"
)

func PublicHandler(w http.ResponseWriter, r *http.Request, appData *i.AppData) {
	url, err := url.Parse(r.RequestURI)
	i.Check(err)
	clientIP, queryIP := i.GetClientIP(url, r, appData.Proxies)

	modeStr := url.Query().Get("mode")
	// Short circuit for IP Only mode
	if modeStr == "ip_only" {
		if !i.CheckAuth(appData.ApiKey, r.Header.Get("X-API-Key")) {
			log.Println(i.LogText(clientIP, i.IPOnly, queryIP, i.Unauthorized))
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, clientIP)
		log.Println(i.LogText(clientIP, i.IPOnly, clientIP, i.GoodAttempt))
		return
	}

	var parsedQueryIP netip.Addr
	badIP := false
	if ip, err := netip.ParseAddr(queryIP); err != nil {
		badIP = true
	} else {
		parsedQueryIP = ip
	}

	var mode i.Mode
	switch modeStr {
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

	data := appData.Databases.GetGeoData(parsedQueryIP, mode, r.UserAgent(), appData.Config.EnableHostName)
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

func PortHandler(w http.ResponseWriter, r *http.Request, appData *i.AppData) {
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
	fmt.Fprint(w, string(jsonData))
}

func HealthHandler(w http.ResponseWriter, r *http.Request, appData *i.AppData) {
	if appData.Databases.Healthy() {
		fmt.Fprint(w, "healthy")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "unhealthy")
	}
}

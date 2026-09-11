package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	i "github.com/SinTan1729/self-ip/internal"
)

var Version = "unknown"

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

	var listenAddr string
	listenPort := "3213"
	if a, flag := os.LookupEnv("SELF_IP_LISTEN_ADDR"); flag {
		listenAddr = a
	}
	if p, flag := os.LookupEnv("SELF_IP_LISTEN_PORT"); flag {
		listenPort = p
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
	listen := fmt.Sprintf("%s:%s", listenAddr, listenPort)
	public := &http.Server{
		Addr:    listen,
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
	defer signal.Stop(sig)

	errc := make(chan error, 2)
	go func() {
		if err := health.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			errc <- fmt.Errorf("health: %w", err)
		}
	}()
	go func() {
		if err := public.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			errc <- fmt.Errorf("public: %w", err)
		}
	}()

	var reason error
	timer := time.NewTimer(100 * time.Millisecond)
	select {
	case reason = <-errc:
		timer.Stop()
	case <-sig:
		timer.Stop()
	case <-timer.C:
		log.Println("Started listening to", listen)

		select {
		case reason = <-errc:
		case <-sig:
		}
	}

	if reason != nil {
		log.Printf("Server failed: %v; shutting down", reason)
	} else {
		log.Println("Shutting down")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := public.Shutdown(ctx); err != nil {
		log.Printf("Server public shutdown: %v", err)
	}
	if err := health.Shutdown(ctx); err != nil {
		log.Printf("Server health shutdown: %v", err)
	}
}

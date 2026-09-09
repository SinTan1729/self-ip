package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	i "github.com/SinTan1729/self-ip/internal"
)

var Version = "unknown"

func basicHandler(w http.ResponseWriter, r *http.Request, databases *i.DatabaseStore, apiKey string) {
	url, err := url.Parse(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}

	clientIP := i.GetClientIP(r)
	customIP := url.Query().Get("ip")
	var queryIP string
	if customIP != "" {
		queryIP = customIP
	} else {
		queryIP = clientIP
	}

	var mode i.Mode
	switch url.Query().Get("mode") {
	case "ip_only":
		mode = i.IPOnly
	case "full":
		mode = i.Full
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

	data := databases.GetGeoData(queryIP, mode)
	if mode != i.IPOnly {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/plain")
	}

	log.Println(i.LogText(clientIP, mode, queryIP, i.GoodAttempt))
	fmt.Fprintf(w, "%s", data)
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

	go databases.ScheduleUpdates()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/json", "/api":
			basicHandler(w, r, databases, apiKey)
		default:
			http.NotFound(w, r)
		}
	})

	log.Println("Server running at http://localhost:3213")
	if err := http.ListenAndServe(":3213", nil); err != nil {
		panic(err)
	}
}

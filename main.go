package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sync"

	"github.com/oschwald/maxminddb-golang/v2"
)

var Version = "unknown"

func getGeoData(rawIP string, dbCity *maxminddb.Reader, dbASN *maxminddb.Reader, mode Mode) []byte {
	ip, err := netip.ParseAddr(rawIP)
	if err != nil {
		log.Fatal()
	}

	var (
		record  CityResponse
		asn     ASNResponse
		cityErr error
		asnErr  error
		wg      sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		cityErr = dbCity.Lookup(ip).Decode(&record)
	}()
	go func() {
		defer wg.Done()
		asnErr = dbASN.Lookup(ip).Decode(&asn)
	}()

	wg.Wait()
	if cityErr != nil || asnErr != nil {
		return nil
	}
	record.IP = rawIP
	record.ASN = asn

	if mode == IPOnly {
		return []byte(rawIP)
	}
	if mode == Default {
		var short shortRecord
		short.IP = rawIP
		short.City = record.City.Names.EN
		if len(record.Subdivisions) > 0 {
			short.Region = &RegionInfo{
				Name:    record.Subdivisions[0].Names.EN,
				ISOCode: record.Subdivisions[0].ISOCode,
			}
		}
		if record.Country.Names.EN != "" {
			short.Country = &RegionInfo{
				Name:    record.Country.Names.EN,
				ISOCode: record.Country.ISOCode,
			}
		}
		if record.Location.AccuracyRadius != 0 {
			short.Location = &LocationInfo{
				Latitude:  record.Location.Latitude,
				Longitude: record.Location.Longitude,
				Postal:    record.Postal.Code,
			}
		}
		short.TimeZone = record.Location.TimeZone
		if record.ASN.AutonomousSystemNumber > 0 {
			short.Organization = fmt.Sprintf("A%d %s",
				record.ASN.AutonomousSystemNumber,
				record.ASN.AutonomousSystemOrganization)
		}

		jsonData, err := json.Marshal(short)
		if err != nil {
			log.Fatal(err)
		}
		return jsonData
	}

	jsonData, err := json.Marshal(record)
	if err != nil {
		log.Fatal(err)
	}
	return jsonData
}

func basicHandler(w http.ResponseWriter, r *http.Request, databases *databaseStore, apiKey string) {
	url, err := url.Parse(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}

	clientIP := getClientIP(r)
	customIP := url.Query().Get("ip")
	var queryIP string
	if customIP != "" {
		queryIP = customIP
	} else {
		queryIP = clientIP
	}

	var mode Mode
	switch url.Query().Get("mode") {
	case "ip_only":
		mode = IPOnly
	case "full":
		mode = Full
	case "", "default":
		mode = Default
	default:
		log.Println(logText(clientIP, mode, queryIP, BadAttempt))
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}

	if !checkAuth(apiKey, r.Header.Get("X-API-Key")) {
		log.Println(logText(clientIP, mode, queryIP, Unauthorized))
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
		http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
		return
	}

	data := databases.getGeoData(queryIP, mode)
	if mode != IPOnly {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/plain")
	}

	log.Println(logText(clientIP, mode, queryIP, GoodAttempt))
	fmt.Fprintf(w, "%s", data)
}

func main() {
	log.SetFlags(0)
	log.SetOutput(new(logWriter))

	if Version == "unknown" {
		log.Println(Blue + "Self IP (dev build)" + Reset)
	} else {
		log.Printf(Blue+"Self IP v%s"+Reset, Version)
	}
	log.Println(Blue + "https://github.com/SinTan1729/self-ip" + Reset)
	log.Println("-----------------")

	getDatabases()
	databases := &databaseStore{}
	if err := databases.reload(); err != nil {
		log.Fatal(err)
	}
	defer func() {
		databases.mu.Lock()
		defer databases.mu.Unlock()
		if databases.dbCity != nil {
			_ = databases.dbCity.Close()
		}
		if databases.dbASN != nil {
			_ = databases.dbASN.Close()
		}
	}()

	apiKey, flag := os.LookupEnv("SELF_IP_API_KEY")
	if !flag {
		log.Fatal("No API key was provided.")
	}

	go scheduleDatabaseUpdates(databases)

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

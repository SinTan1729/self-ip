package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"

	"github.com/oschwald/maxminddb-golang/v2"
)

var Version = "(Dev)"

func getGeoData(rawIP string, dbCity *maxminddb.Reader, dbASN *maxminddb.Reader, mode Mode) []byte {
	ip, err := netip.ParseAddr(rawIP)
	if err != nil {
		log.Fatal()
	}

	var record CityResponse
	var asn ASNResponse
	err = dbCity.Lookup(ip).Decode(&record)
	if err != nil {
		return nil
	}
	err = dbASN.Lookup(ip).Decode(&asn)
	if err != nil {
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
			short.Region.Name = record.Subdivisions[0].Names.EN
			short.Region.ISOCode = record.Subdivisions[0].ISOCode
		}
		short.Country.Name = record.Country.Names.EN
		short.Country.ISOCode = record.Country.ISOCode
		short.Location.Latitude = record.Location.Latitude
		short.Location.Longitude = record.Location.Longitude
		short.Location.Postal = record.Postal.Code
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
	var ip string
	if customIP != "" {
		ip = customIP
	} else {
		ip = clientIP
	}

	mode := Default
	modeStr := "Default"
	switch url.Query().Get("mode") {
	case "ip_only":
		mode = IPOnly
		modeStr = "IPOnly"
	case "full":
		mode = Full
		modeStr = "Full"
	}
	if !checkAuth(apiKey, r.Header.Get("X-API-Key")) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusUnauthorized)
		log.Printf("!!! Unauthorized access attempted from %s querying %s in %s mode", clientIP, ip, modeStr)
		fmt.Fprintf(w, "Unauthorized")
		return
	}

	data := databases.getGeoData(ip, mode)
	if mode != IPOnly {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/plain")
	}

	log.Printf("--- Accessed from %s querying %s in %s mode", clientIP, ip, modeStr)
	fmt.Fprintf(w, "%s", data)
}

func main() {
	log.SetFlags(0)
	log.SetOutput(new(logWriter))
	getDatabases()

	fmt.Printf("Self IP v%s\n", Version)
	fmt.Println("https://github.com/SinTan1729/self-ip")
	fmt.Println("-----------------\n")

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
		basicHandler(w, r, databases, apiKey)
	})
	fmt.Println("Server running at http://localhost:3213")
	if err := http.ListenAndServe(":3213", nil); err != nil {
		panic(err)
	}
}

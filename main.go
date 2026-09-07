package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

func getGeoData(rawIP string, dbCity *maxminddb.Reader, dbASN *maxminddb.Reader, full bool) []byte {
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

	if !full {
		var short shortRecord
		short.IP = rawIP
		short.City = record.City.Names.EN
		short.Country.Name = record.Country.Names.EN
		short.Country.ISOCode = record.Country.ISOCode
		short.Location.Latitude = record.Location.Latitude
		short.Location.Longitude = record.Location.Longitude
		short.Location.Postal = record.Postal.Code
		short.TimeZone = record.Location.TimeZone
		short.Organization = fmt.Sprintf("A%d %s",
			record.ASN.AutonomousSystemNumber,
			record.ASN.AutonomousSystemOrganization)

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

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (common behind proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to the direct remote address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

func basicHandler(w http.ResponseWriter, r *http.Request, dbCity *maxminddb.Reader, dbASN *maxminddb.Reader) {
	url, err := url.Parse(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}

	customIP := url.Query().Get("ip")
	var ip string
	if customIP != "" {
		ip = customIP
	} else {
		ip = getClientIP(r)
	}

	full := url.Query().Get("full")
	data := getGeoData(ip, dbCity, dbASN, full == "true")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", data)
}

func main() {
	dbCity, err := maxminddb.Open("GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer dbCity.Close()
	dbASN, err := maxminddb.Open("GeoLite2-ASN.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer dbASN.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		basicHandler(w, r, dbCity, dbASN)
	})
	fmt.Println("Server running at http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

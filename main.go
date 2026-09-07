package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/oschwald/maxminddb-golang/v2"
)

func getDatabases() {
	err := os.MkdirAll("./maxmind-databases", 0755)
	check(err)

	curVer := semver.MustParse("0.0.0")
	f, err := os.ReadFile("./maxmind-databases/version")
	if err == nil {
		curVer, err = semver.NewVersion(string(f))
		if err != nil {
			curVer = semver.MustParse("0.0.0")
		}
	}

	_, err = os.Stat("./maxmind-databases/GeoLite2-City.mmdb")
	if err != nil {
		curVer = semver.MustParse("0.0.0")
	}
	_, err = os.Stat("./maxmind-databases/GeoLite2-ASN.mmdb")
	if err != nil {
		curVer = semver.MustParse("0.0.0")
	}

	resp, err := http.Get("https://api.github.com/repos/P3TERX/GeoLite.mmdb/releases/latest")
	check(err)
	defer resp.Body.Close()
	type Release struct {
		TagName string `json:"tag_name"`
	}
	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		panic(err)
	}
	newVer, err := semver.NewVersion(release.TagName)
	check(err)

	if newVer.GreaterThan(curVer) {
		fmt.Println("New version of databases available:", newVer)
	} else {
		fmt.Println("Already have the latest databases:", curVer)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	fmt.Println("Downloading GeoLite2-City.mmdb")
	err = downloadFile(
		ctx,
		"https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb",
		"./maxmind-databases/GeoLite2-City.mmdb.tmp",
	)
	check(err)
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	fmt.Println("Downloading GeoLite2-ASN.mmdb")
	err = downloadFile(
		ctx,
		"https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb",
		"./maxmind-databases/GeoLite2-ASN.mmdb.tmp",
	)
	check(err)
	err = os.WriteFile("./maxmind-databases/version.tmp", []byte(newVer.Original()), 0644)
	check(err)

	files := []string{"GeoLite2-City.mmdb", "GeoLite2-ASN.mmdb", "version"}
	for _, file := range files {
		err = os.Rename(fmt.Sprintf("./maxmind-databases/%s.tmp", file), fmt.Sprintf("./maxmind-databases/%s", file))
		check(err)
	}
	fmt.Println("Databases updated to version:", newVer)
}

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

func basicHandler(w http.ResponseWriter, r *http.Request, dbCity *maxminddb.Reader, dbASN *maxminddb.Reader, apiKey string) {
	if !checkAuth(apiKey, r.Header.Get("X-API-Key")) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "Unauthorized")
		return
	}

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
	getDatabases()

	dbCity, err := maxminddb.Open("./maxmind-databases/GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer dbCity.Close()
	dbASN, err := maxminddb.Open("./maxmind-databases/GeoLite2-ASN.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer dbASN.Close()

	apiKey, flag := os.LookupEnv("SELF_IP_API_KEY")
	if !flag {
		log.Fatal("No API key was provided.")
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		basicHandler(w, r, dbCity, dbASN, apiKey)
	})
	fmt.Println("Server running at http://localhost:3213")
	err = http.ListenAndServe(":3213", nil)
	if err != nil {
		panic(err)
	}
}

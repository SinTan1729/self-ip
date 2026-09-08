package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
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
	type Asset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"`
	}
	type Release struct {
		TagName string  `json:"tag_name"`
		Assets  []Asset `json:"assets"`
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

	assets := make(map[string]Asset, len(release.Assets))
	for _, asset := range release.Assets {
		assets[asset.Name] = asset
	}

	downloadAndVerify := func(name string) error {
		asset, ok := assets[name]
		if !ok {
			return fmt.Errorf("release %s does not contain asset %s", release.TagName, name)
		}
		expected, ok := strings.CutPrefix(asset.Digest, "sha256:")
		if !ok || len(expected) != sha256.Size*2 {
			return fmt.Errorf("release %s has no valid SHA-256 digest for %s", release.TagName, name)
		}
		expectedBytes, err := hex.DecodeString(expected)
		if err != nil {
			return fmt.Errorf("invalid SHA-256 digest for %s: %w", name, err)
		}

		tmp := fmt.Sprintf("./maxmind-databases/%s.tmp", name)
		verified := false
		defer func() {
			if !verified {
				_ = os.Remove(tmp)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		fmt.Println("Downloading", name)
		if err := downloadFile(ctx, asset.BrowserDownloadURL, tmp); err != nil {
			return err
		}

		f, err := os.Open(tmp)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if !bytes.Equal(h.Sum(nil), expectedBytes) {
			return fmt.Errorf("SHA-256 verification failed for %s", name)
		}
		fmt.Println("Verified SHA-256 for", name)
		verified = true
		return nil
	}

	check(downloadAndVerify("GeoLite2-City.mmdb"))
	check(downloadAndVerify("GeoLite2-ASN.mmdb"))
	err = os.WriteFile("./maxmind-databases/version.tmp", []byte(newVer.Original()), 0644)
	check(err)

	files := []string{"GeoLite2-City.mmdb", "GeoLite2-ASN.mmdb", "version"}
	for _, file := range files {
		err = os.Rename(fmt.Sprintf("./maxmind-databases/%s.tmp", file), fmt.Sprintf("./maxmind-databases/%s", file))
		check(err)
	}
	fmt.Println("Databases updated to version:", newVer)
}

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

	data := getGeoData(ip, dbCity, dbASN, mode)
	if mode != IPOnly {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/plain")
	}

	log.Printf("--- Accessed from %s querying %s in %s mode", clientIP, ip, modeStr)
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

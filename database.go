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
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/oschwald/maxminddb-golang/v2"
)

type databaseStore struct {
	mu     sync.RWMutex
	dbCity *maxminddb.Reader
	dbASN  *maxminddb.Reader
}

func (d *databaseStore) getGeoData(rawIP string, mode Mode) []byte {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return getGeoData(rawIP, d.dbCity, d.dbASN, mode)
}

func (d *databaseStore) reload() error {
	type result struct {
		city *maxminddb.Reader
		asn  *maxminddb.Reader
		err  error
	}

	cityCh := make(chan result, 1)
	asnCh := make(chan result, 1)
	go func() {
		db, err := maxminddb.Open(
			"./maxmind-databases/GeoLite2-City.mmdb",
		)
		cityCh <- result{city: db, err: err}
	}()
	go func() {
		db, err := maxminddb.Open(
			"./maxmind-databases/GeoLite2-ASN.mmdb",
		)
		asnCh <- result{asn: db, err: err}
	}()

	cityResult := <-cityCh
	asnResult := <-asnCh
	if cityResult.err != nil {
		if asnResult.asn != nil {
			_ = asnResult.asn.Close()
		}
		return cityResult.err
	}
	if asnResult.err != nil {
		_ = cityResult.city.Close()
		return asnResult.err
	}

	d.mu.Lock()
	oldCity, oldASN := d.dbCity, d.dbASN
	d.dbCity = cityResult.city
	d.dbASN = asnResult.asn
	d.mu.Unlock()

	if oldCity != nil {
		_ = oldCity.Close()
	}
	if oldASN != nil {
		_ = oldASN.Close()
	}

	return nil
}

func getDatabases() {
	log.Println("Checking for database updates.")
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
		log.Println("New version of databases available:", newVer)
	} else {
		log.Println("Already have the latest databases:", curVer)
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
		log.Println("Downloading", name)
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
		log.Println("Verified SHA-256 for", name)
		verified = true
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	filesToDownload := []string{
		"GeoLite2-City.mmdb",
		"GeoLite2-ASN.mmdb",
	}

	for _, name := range filesToDownload {
		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			if err := downloadAndVerify(name); err != nil {
				errCh <- err
			}
		}(name)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		check(err)
	}
	err = os.WriteFile("./maxmind-databases/version.tmp", []byte(newVer.Original()), 0644)
	check(err)
	files := []string{"GeoLite2-City.mmdb", "GeoLite2-ASN.mmdb", "version"}
	for _, file := range files {
		err = os.Rename(fmt.Sprintf("./maxmind-databases/%s.tmp", file), fmt.Sprintf("./maxmind-databases/%s", file))
		check(err)
	}

	log.Println("Databases updated to version:", newVer)
}

func scheduleDatabaseUpdates(databases *databaseStore) {
	for {
		now := time.Now().Add(time.Hour)
		next := time.Date(now.Year(), now.Month(), now.Day(), 7, rand.IntN(10)-5, rand.IntN(60)-30, 0, time.UTC)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}

		log.Printf("Next database update scheduled for %s", next.Local().Format(time.RFC3339))
		time.Sleep(time.Until(next))

		log.Println("Running scheduled database update")
		getDatabases()
		if err := databases.reload(); err != nil {
			log.Printf("Database reload failed: %v", err)
			continue
		}
		log.Println("Scheduled database update completed")
	}
}

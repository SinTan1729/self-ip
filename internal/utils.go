package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/matthewhartstonge/argon2"
	"github.com/oschwald/maxminddb-golang/v2"
)

func Check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func downloadFile(ctx context.Context, url, filename string) error {
	client := &http.Client{
		Timeout: 0, // no overall timeout; context controls cancellation
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func GetClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

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

func CheckAuth(key string, provided string) bool {
	ok, err := argon2.VerifyEncoded([]byte(provided), []byte(key))
	if err != nil {
		return false
	}
	return ok
}

func (writer LogWriter) Write(bytes []byte) (int, error) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	return fmt.Print(
		Grey + "[" + timestamp + "]" + Reset + " " + string(bytes),
	)
}

func LogText(clientIP string, mode Mode, queryIP string, attemptType uint) string {
	var prefix string
	var color string
	var modeText string
	var queryText string

	switch attemptType {
	case GoodAttempt:
		prefix = "Accessed"
		color = Green
	case Unauthorized:
		prefix = "Unauthorized attempt"
		color = Red
	case BadAttempt:
		prefix = "Bad request"
		color = Red
	}

	switch mode {
	case Full:
		modeText = ", mode: Full"
	case IPOnly:
		modeText = ", mode: IPOnly"
	}

	if queryIP != clientIP {
		queryText = ", query: " + queryIP
	}

	return fmt.Sprintf("%s%s from %s%s%s%s", color, prefix, clientIP, modeText, queryText, Reset)
}

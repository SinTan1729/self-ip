package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"slices"
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

func GetClientIP(url *url.URL, r *http.Request) (string, string) {
	customIP := url.Query().Get("ip")
	var queryIP string
	var clientIP string

	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		clientIP = strings.TrimSpace(ips[0])
	}

	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}

	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = ip
	}

	if customIP != "" {
		queryIP = customIP
	} else {
		queryIP = clientIP
	}
	return clientIP, queryIP
}

func calcIPDecimal(rawIP string) *JSONBigInt {
	ip := net.ParseIP(rawIP)
	bigInt := big.NewInt(0)
	if v4 := ip.To4(); v4 != nil {
		return (*JSONBigInt)(bigInt.SetBytes(v4))
	} else {
		return (*JSONBigInt)(bigInt.SetBytes(ip.To16()))
	}
}

func getGeoData(rawIP string, dbCity *maxminddb.Reader, dbASN *maxminddb.Reader, mode Mode, uAgent string) []byte {
	ip, err := netip.ParseAddr(rawIP)
	if err != nil {
		log.Fatal()
	}

	var (
		record  fullResponse
		asn     asnResponse
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
	uAgentParts := strings.SplitN(uAgent, " ", 2)
	var uAgentComment string
	if len(uAgentParts) > 1 {
		uAgentComment = uAgentParts[1]
	}
	uAgentMainParts := strings.SplitN(uAgentParts[0], "/", 2)
	var uAgentVersion string
	if len(uAgentMainParts) > 1 {
		uAgentVersion = uAgentMainParts[1]
	}
	record.UserAgent = userAgent{
		Product:  uAgentMainParts[0],
		Version:  uAgentVersion,
		Comment:  uAgentComment,
		RawValue: uAgent,
	}

	switch mode {
	case IPOnly:
		return []byte(rawIP)

	case Default:
		var res defaultResponse
		res.IP = rawIP
		res.City = record.City.Names.EN
		if len(record.Subdivisions) > 0 {
			res.Region = &regionInfo{
				Name:    record.Subdivisions[0].Names.EN,
				ISOCode: record.Subdivisions[0].ISOCode,
			}
		}
		if record.Country.Names.EN != "" {
			res.Country = &regionInfo{
				Name:    record.Country.Names.EN,
				ISOCode: record.Country.ISOCode,
			}
		}
		if record.Location.AccuracyRadius != 0 {
			res.Location = &locationInfo{
				Latitude:  record.Location.Latitude,
				Longitude: record.Location.Longitude,
				Postal:    record.Postal.Code,
			}
		}
		res.TimeZone = record.Location.TimeZone
		if record.ASN.AutonomousSystemNumber > 0 {
			res.Organization = fmt.Sprintf("A%d %s",
				record.ASN.AutonomousSystemNumber,
				record.ASN.AutonomousSystemOrganization)
		}

		jsonData, err := json.Marshal(res)
		if err != nil {
			log.Fatal(err)
		}
		return jsonData

	case EchoIP:
		var res echoIPResponse
		res.IP = rawIP
		res.IPDecimal = calcIPDecimal(rawIP)
		res.Country = record.Country.Names.EN
		res.CountryISO = record.Country.ISOCode
		res.CountryEU = slices.Contains(EUCountries, res.CountryISO)
		if len(record.Subdivisions) > 0 {
			res.RegionName = record.Subdivisions[0].Names.EN
			res.RegionCode = record.Subdivisions[0].ISOCode
		}
		res.MetroCode = record.Location.MetroCode
		res.City = record.City.Names.EN
		if record.Location.AccuracyRadius != 0 {
			res.Latitude = record.Location.Latitude
			res.Longitude = record.Location.Longitude
			res.TimeZone = record.Location.TimeZone
			res.ZipCode = record.Postal.Code
		}
		if record.ASN.AutonomousSystemNumber > 0 {
			res.ASN = fmt.Sprintf("A%d", record.ASN.AutonomousSystemNumber)
			res.ASNOrg = record.ASN.AutonomousSystemOrganization
		}
		res.UserAgent = &record.UserAgent

		jsonData, err := json.Marshal(res)
		if err != nil {
			log.Fatal(err)
		}
		return jsonData

	default: // mode = Full
		names, err := net.LookupAddr(rawIP)
		if err == nil && len(names) > 0 {
			record.HostName = strings.TrimRight(names[0], ".")
		}
		record.IPDecimal = calcIPDecimal(rawIP)
		jsonData, err := json.Marshal(record)
		if err != nil {
			log.Fatal(err)
		}
		return jsonData
	}
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
		modeText = ", mode: IP only"
	case EchoIP:
		modeText = ", mode: echoip"
	case PortCheck:
		modeText = ", mode: Port check"
	}

	if queryIP != clientIP {
		queryText = ", query: " + queryIP
	}

	return fmt.Sprintf("%s%s from %s%s%s%s", color, prefix, clientIP, modeText, queryText, Reset)
}

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

func isTrustedProxy(rawIP string, prefixes []netip.Prefix) bool {
	ip := netip.MustParseAddr(rawIP).Unmap()
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func GetClientIP(url *url.URL, r *http.Request, trustedProxies []netip.Prefix) (string, string) {
	customIP := url.Query().Get("ip")
	var queryIP string
	var clientIP string
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = ip
	}

	if isTrustedProxy(clientIP, trustedProxies) {
		forwarded := r.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			ips := strings.Split(forwarded, ",")
			clientIP = strings.TrimSpace(ips[0])
		} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			clientIP = realIP
		}
	}
	if customIP != "" {
		queryIP = customIP
	} else {
		queryIP = clientIP
	}
	return clientIP, queryIP
}

func calcIPDecimal(ip netip.Addr) *JSONBigInt {
	bigInt := big.NewInt(0)
	if ip.Is4() {
		b := ip.As4()
		return (*JSONBigInt)(bigInt.SetBytes(b[:]))
	} else {
		b := ip.As16()
		return (*JSONBigInt)(bigInt.SetBytes(b[:]))
	}
}

func getGeoData(ip netip.Addr, dbCity *maxminddb.Reader, dbASN *maxminddb.Reader, mode Mode, uAgent string) []byte {
	// IP Only mode doesn't reach here
	getAgent := func(uAgent string) userAgent {
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

		return userAgent{
			Product:  uAgentMainParts[0],
			Version:  uAgentVersion,
			Comment:  uAgentComment,
			RawValue: uAgent,
		}
	}

	var (
		asn     asnResponse
		cityErr error
		asnErr  error
		wg      sync.WaitGroup
	)

	jsonDispatch := func(data any) []byte {
		jsonData, err := json.Marshal(data)
		Check(err)
		return jsonData
	}

	if mode == Full {
		var record fullResponse
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

		record.IP = ip
		record.ASN = asn
		record.UserAgent = getAgent(uAgent)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if names, err := net.DefaultResolver.LookupAddr(ctx, ip.String()); err == nil && len(names) > 0 {
			record.HostName = strings.TrimRight(names[0], ".")
		}
		record.IPDecimal = calcIPDecimal(ip)
		if record.Country.ISOCode != "" {
			eu := slices.Contains(EUCountries, record.Country.ISOCode)
			record.Country.InEU = &eu
		}

		return jsonDispatch(record)
	}

	var record intermediateData

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
	record.ASN = asn

	if mode == Short {
		var res shortResponse
		res.IP = ip
		res.City = record.City.Names.EN
		if len(record.Region) > 0 {
			res.Region = record.Region[0].Names.EN
		}
		res.Country = record.Country.Names.EN
		res.TimeZone = record.Location.TimeZone

		return jsonDispatch(res)
	}

	if mode == EchoIP {
		var res echoIPResponse
		res.IP = ip
		res.IPDecimal = calcIPDecimal(ip)
		res.Country = record.Country.Names.EN
		res.CountryISO = record.Country.ISOCode
		if res.CountryISO != "" {
			eu := slices.Contains(EUCountries, res.CountryISO)
			res.CountryEU = &eu
		}
		if len(record.Region) > 0 {
			res.RegionName = record.Region[0].Names.EN
			res.RegionCode = record.Region[0].ISOCode
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
		agent := getAgent(uAgent)
		res.UserAgent = &agent

		return jsonDispatch(res)
	}

	// Default mode
	var res defaultResponse
	res.IP = ip
	res.City = record.City.Names.EN
	if len(record.Region) > 0 {
		res.Region = &regionInfo{
			Name:    record.Region[0].Names.EN,
			ISOCode: record.Region[0].ISOCode,
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

	return jsonDispatch(res)
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
	case Short:
		modeText = ", mode: Short"
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

func ParseTrustedProxies(value string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, s := range strings.Split(value, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			ip, err := netip.ParseAddr(s)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy IP %q: %w", s, err)
			}
			if ip.Is4() {
				s += "/32"
			} else {
				s += "/128"
			}
		}

		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy subnet %q: %w", s, err)
		}

		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func GetOwnIPs() []netip.Addr {
	type networkKey struct{}
	url := "https://ifconfig.co/ip"
	var addrs []netip.Addr

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				network, _ := ctx.Value(networkKey{}).(string)
				if network == "" {
					network = "tcp"
				}
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}

	get := func(network string) (netip.Addr, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ctx = context.WithValue(ctx, networkKey{}, network)

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		var ip string
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			if b, err := io.ReadAll(resp.Body); err == nil {
				ip = string(b)
			}
		}
		return netip.ParseAddr(strings.TrimSuffix(ip, "\n"))
	}

	if ipv4, err := get("tcp4"); err == nil {
		addrs = append(addrs, ipv4)
	}
	if ipv6, err := get("tcp6"); err == nil {
		addrs = append(addrs, ipv6)
	}

	return addrs
}

func PrettyPrintArray[T any](v []T) string {
	var b strings.Builder
	for i, x := range v {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprint(&b, x)
	}
	return b.String()
}

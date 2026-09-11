package internal

import (
	"math/big"
	"net/netip"
)

type Mode uint

const (
	Default   Mode = 0
	IPOnly    Mode = 1
	Short     Mode = 2
	EchoIP    Mode = 3
	Full      Mode = 4
	PortCheck Mode = 5
)

type AppData struct {
	ApiKey    string
	Databases *DatabaseStore
	Config    struct {
		EnableHostName bool
	}
	Proxies []netip.Prefix
	OwnIP   []netip.Addr
}

type names struct {
	DE   string `maxminddb:"de" json:"de,omitempty"`
	EN   string `maxminddb:"en" json:"en,omitempty"`
	ES   string `maxminddb:"es" json:"es,omitempty"`
	FR   string `maxminddb:"fr" json:"fr,omitempty"`
	JA   string `maxminddb:"ja" json:"ja,omitempty"`
	PTBR string `maxminddb:"pt-BR" json:"pt-BR,omitempty"`
	RU   string `maxminddb:"ru" json:"ru,omitempty"`
	ZHCN string `maxminddb:"zh-CN" json:"zh-CN,omitempty"`
}
type city struct {
	GeoNameID uint64 `maxminddb:"geoname_id" json:"geoname_id,omitempty"`
	Names     names  `maxminddb:"names" json:"names,omitempty"`
}
type continent struct {
	Code      string `maxminddb:"code" json:"code,omitempty"`
	GeoNameID uint64 `maxminddb:"geoname_id" json:"geoname_id,omitempty"`
	Names     names  `maxminddb:"names" json:"names,omitempty"`
}
type country struct {
	GeoNameID uint64 `maxminddb:"geoname_id" json:"geoname_id,omitempty"`
	ISOCode   string `maxminddb:"iso_code" json:"iso_code,omitempty"`
	Names     names  `maxminddb:"names" json:"names,omitempty"`
	InEU      *bool  `json:"in_eu,omitempty"`
}
type location struct {
	AccuracyRadius uint64  `maxminddb:"accuracy_radius" json:"accuracy_radius,omitempty"`
	Latitude       float64 `maxminddb:"latitude" json:"latitude,omitempty"`
	Longitude      float64 `maxminddb:"longitude" json:"longitude,omitempty"`
	MetroCode      uint64  `maxminddb:"metro_code" json:"metro_code,omitempty"`
	TimeZone       string  `maxminddb:"time_zone" json:"time_zone,omitempty"`
}
type postal struct {
	Code string `maxminddb:"code" json:"code,omitempty"`
}
type subdivision struct {
	GeoNameID uint64 `maxminddb:"geoname_id" json:"geoname_id,omitempty"`
	ISOCode   string `maxminddb:"iso_code" json:"iso_code,omitempty"`
	Names     names  `maxminddb:"names" json:"names,omitempty"`
}
type asnResponse struct {
	AutonomousSystemNumber       uint64 `maxminddb:"autonomous_system_number" json:"number,omitempty"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization" json:"organization,omitempty"`
}
type userAgent struct {
	Product  string `json:"product,omitempty"`
	Version  string `json:"version,omitempty"`
	Comment  string `json:"comment,omitempty"`
	RawValue string `json:"raw_value,omitempty"`
}

type JSONBigInt big.Int

func (n JSONBigInt) MarshalJSON() ([]byte, error) {
	return []byte((*big.Int)(&n).String()), nil
}

type fullResponse struct {
	IP                netip.Addr    `json:"ip"`
	IPDecimal         *JSONBigInt   `json:"ip_decimal"`
	HostName          string        `json:"hostname,omitempty"`
	City              city          `maxminddb:"city" json:"city,omitempty"`
	Country           country       `maxminddb:"country" json:"country,omitempty"`
	Continent         continent     `maxminddb:"continent" json:"continent,omitempty"`
	Location          location      `maxminddb:"location" json:"location,omitempty"`
	Postal            postal        `maxminddb:"postal" json:"postal,omitempty"`
	RegisteredCountry country       `maxminddb:"registered_country" json:"registered_country,omitempty"`
	Subdivisions      []subdivision `maxminddb:"subdivisions" json:"subdivisions,omitempty"`
	ASN               asnResponse   `json:"asn,omitempty"`
	UserAgent         userAgent     `json:"user_agent,omitempty"`
}

type intermediateData struct {
	IP        netip.Addr
	IPDecimal *JSONBigInt
	City      struct {
		Names struct {
			EN string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"city"`
	Country struct {
		Names struct {
			EN string `maxminddb:"en"`
		} `maxminddb:"names"`
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	Location location `maxminddb:"location"`
	Postal   postal   `maxminddb:"postal"`
	Region   []struct {
		Names struct {
			EN string `maxminddb:"en"`
		} `maxminddb:"names"`
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"subdivisions,maxsize:1"`
	ASN asnResponse
}

type regionInfo struct {
	Name    string `json:"name,omitempty"`
	ISOCode string `json:"iso,omitempty"`
}
type locationInfo struct {
	Latitude  float64 `json:"lat,omitempty"`
	Longitude float64 `json:"long,omitempty"`
	Postal    string  `json:"postal,omitempty"`
}
type defaultResponse struct {
	IP           netip.Addr    `json:"ip"`
	City         string        `json:"city,omitempty"`
	Region       *regionInfo   `json:"region,omitempty"`
	Country      *regionInfo   `json:"country,omitempty"`
	Location     *locationInfo `json:"location,omitempty"`
	TimeZone     string        `json:"tz,omitempty"`
	Organization string        `json:"org,omitempty"`
}

type echoIPResponse struct {
	IP         netip.Addr  `json:"ip"`
	IPDecimal  *JSONBigInt `json:"ip_decimal"`
	Country    string      `json:"country,omitempty"`
	CountryISO string      `json:"country_iso,omitempty"`
	CountryEU  *bool       `json:"country_eu,omitempty"`
	RegionName string      `json:"region_name,omitempty"`
	RegionCode string      `json:"region_code,omitempty"`
	MetroCode  uint64      `json:"metro_code,omitempty"`
	ZipCode    string      `json:"zip_code,omitempty"`
	City       string      `json:"city,omitempty"`
	Latitude   float64     `json:"latitude,omitempty"`
	Longitude  float64     `json:"longitude,omitempty"`
	TimeZone   string      `json:"time_zone,omitempty"`
	ASN        string      `json:"asn,omitempty"`
	ASNOrg     string      `json:"asn_org,omitempty"`
	UserAgent  *userAgent  `json:"user_agent,omitempty"`
}

type shortResponse struct {
	IP       netip.Addr `json:"ip"`
	City     string     `json:"city,omitempty"`
	Region   string     `json:"region,omitempty"`
	Country  string     `json:"country,omitempty"`
	TimeZone string     `json:"tz,omitempty"`
}

type PortStatus string

const (
	PortOpen        PortStatus = "open"        // TCP handshake completed
	PortRefused     PortStatus = "refused"     // RST (nothing listening)
	PortTimeout     PortStatus = "timeout"     // Packet dropped
	PortUnreachable PortStatus = "unreachable" // Host/Network Unreachable
	PortUnknown     PortStatus = "unknown"     // Unhandled error
)

type PortResponse struct {
	IP        netip.Addr `json:"ip"`
	Port      uint16     `json:"port"`
	Reachable bool       `json:"reachable"`
	Status    PortStatus `json:"status"`
}

type LogWriter struct {
}

const (
	Reset = "\033[0m"
	Red   = "\033[31m"
	Grey  = "\033[90m"
	Blue  = "\033[34m"
	Green = "\033[32m"
)

const (
	GoodAttempt  = 0
	BadAttempt   = 1
	Unauthorized = 2
)

var EUCountries = []string{
	"AT", "BE", "BG", "HR", "CY", "CZ", "DK", "EE", "FI", "FR",
	"DE", "GR", "HU", "IE", "IT", "LV", "LT", "LU", "MT", "NL",
	"PL", "PT", "RO", "SK", "SI", "ES", "SE",
}

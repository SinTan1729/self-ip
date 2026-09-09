package internal

type Mode uint

const (
	Default Mode = 0
	IPOnly  Mode = 1
	Full    Mode = 2
)

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
type cityResponse struct {
	IP                string        `json:"ip"`
	City              city          `maxminddb:"city" json:"city,omitempty"`
	Continent         continent     `maxminddb:"continent" json:"continent,omitempty"`
	Country           country       `maxminddb:"country" json:"country,omitempty"`
	Location          location      `maxminddb:"location" json:"location,omitempty"`
	Postal            postal        `maxminddb:"postal" json:"postal,omitempty"`
	RegisteredCountry country       `maxminddb:"registered_country" json:"registered_country,omitempty"`
	Subdivisions      []subdivision `maxminddb:"subdivisions" json:"subdivisions,omitempty"`
	ASN               asnResponse   `json:"asn,omitempty"`
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
type shortRecord struct {
	IP           string        `json:"ip"`
	City         string        `json:"city,omitempty"`
	Region       *regionInfo   `json:"region,omitempty"`
	Country      *regionInfo   `json:"country,omitempty"`
	Location     *locationInfo `json:"location,omitempty"`
	TimeZone     string        `json:"tz,omitempty"`
	Organization string        `json:"org,omitempty"`
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

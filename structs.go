package main

type Mode uint

const (
	Default Mode = 0
	IPOnly  Mode = 1
	Full    Mode = 2
)

type Names struct {
	DE   string `maxminddb:"de" json:"de,omitempty"`
	EN   string `maxminddb:"en" json:"en,omitempty"`
	ES   string `maxminddb:"es" json:"es,omitempty"`
	FR   string `maxminddb:"fr" json:"fr,omitempty"`
	JA   string `maxminddb:"ja" json:"ja,omitempty"`
	PTBR string `maxminddb:"pt-BR" json:"pt-BR,omitempty"`
	RU   string `maxminddb:"ru" json:"ru,omitempty"`
	ZHCN string `maxminddb:"zh-CN" json:"zh-CN,omitempty"`
}

type City struct {
	GeoNameID uint64 `maxminddb:"geoname_id" json:"geoname_id,omitempty"`
	Names     Names  `maxminddb:"names" json:"names,omitempty"`
}

type Continent struct {
	Code      string `maxminddb:"code" json:"code,omitempty"`
	GeoNameID uint64 `maxminddb:"geoname_id" json:"geoname_id,omitempty"`
	Names     Names  `maxminddb:"names" json:"names,omitempty"`
}

type Country struct {
	GeoNameID uint64 `maxminddb:"geoname_id" json:"geoname_id,omitempty"`
	ISOCode   string `maxminddb:"iso_code" json:"iso_code,omitempty"`
	Names     Names  `maxminddb:"names" json:"names,omitempty"`
}

type Location struct {
	AccuracyRadius uint64  `maxminddb:"accuracy_radius" json:"accuracy_radius,omitempty"`
	Latitude       float64 `maxminddb:"latitude" json:"latitude,omitempty"`
	Longitude      float64 `maxminddb:"longitude" json:"longitude,omitempty"`
	MetroCode      uint64  `maxminddb:"metro_code" json:"metro_code,omitempty"`
	TimeZone       string  `maxminddb:"time_zone" json:"time_zone,omitempty"`
}

type Postal struct {
	Code string `maxminddb:"code" json:"code,omitempty"`
}

type Subdivision struct {
	GeoNameID uint64 `maxminddb:"geoname_id" json:"geoname_id,omitempty"`
	ISOCode   string `maxminddb:"iso_code" json:"iso_code,omitempty"`
	Names     Names  `maxminddb:"names" json:"names,omitempty"`
}

type ASNResponse struct {
	AutonomousSystemNumber       uint64 `maxminddb:"autonomous_system_number" json:"number,omitempty"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization" json:"organization,omitempty"`
}

type CityResponse struct {
	IP                string        `json:"ip"`
	City              City          `maxminddb:"city" json:"city,omitempty"`
	Continent         Continent     `maxminddb:"continent" json:"continent,omitempty"`
	Country           Country       `maxminddb:"country" json:"country,omitempty"`
	Location          Location      `maxminddb:"location" json:"location,omitempty"`
	Postal            Postal        `maxminddb:"postal" json:"postal,omitempty"`
	RegisteredCountry Country       `maxminddb:"registered_country" json:"registered_country,omitempty"`
	Subdivisions      []Subdivision `maxminddb:"subdivisions" json:"subdivisions,omitempty"`
	ASN               ASNResponse   `json:"asn,omitempty"`
}

type shortRecord struct {
	IP      string `json:"ip"`
	City    string `json:"city,omitempty"`
	Country struct {
		Name    string `json:"name,omitempty"`
		ISOCode string `json:"iso,omitempty"`
	} `json:"country,omitempty"`
	Location struct {
		Latitude  float64 `json:"lat,omitempty"`
		Longitude float64 `json:"long,omitempty"`
		Postal    string  `json:"postal,omitempty"`
	} `json:"location,omitempty"`
	TimeZone     string `json:"tz,omitempty"`
	Organization string `json:"org,omitempty"`
}

type logWriter struct {
}

package stats

import "net/netip"

// cityDatabase is the City database's name, in its files, metrics and logs.
const cityDatabase = "city"

// DefaultCityDatabaseURL is the City database's URL template by default:
// DB-IP's City Lite monthly release.
const DefaultCityDatabaseURL = "https://download.db-ip.com/free/dbip-city-lite-" +
	placeholderYear + "-" + placeholderMonth + ".mmdb.gz"

// cityRecord is the part of a City database record goblog reads: never the
// city or coordinates.
type cityRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	// Subdivisions are largest first. DB-IP's City Lite has only their
	// English names, no codes, so goblog reads only the names, even from
	// databases that have codes.
	Subdivisions []struct {
		Names struct {
			English string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
}

// location is where a Visitor is, no finer than state or province.
type location struct {
	// country is the ISO 3166-1 alpha-2 code, such as "US", or "".
	country string
	// region is the largest subdivision's English name, such as "New York",
	// or "".
	region string
}

// locationOf is where ip is, from db, or the zero location if db doesn't
// know (or isn't loaded).
func locationOf(db *database, ip netip.Addr) location {
	var rec cityRecord
	if db == nil || !db.lookup(ip, &rec) {
		return location{}
	}
	loc := location{country: rec.Country.ISOCode}
	if len(rec.Subdivisions) > 0 {
		loc.region = rec.Subdivisions[0].Names.English
	}
	return loc
}

package stats

import "net/netip"

// asnDatabase is the ASN database's name, in its files, metrics and logs.
const asnDatabase = "asn"

// DefaultASNDatabaseURL is the ASN database's URL template by default:
// DB-IP's ASN Lite monthly release.
const DefaultASNDatabaseURL = "https://download.db-ip.com/free/dbip-asn-lite-" +
	placeholderYear + "-" + placeholderMonth + ".mmdb.gz"

// asnRecord is an ASN database record. DB-IP's ASN Lite uses GeoLite2
// ASN's field names.
type asnRecord struct {
	Number uint32 `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

// network is the autonomous system an IP is in.
type network struct {
	// asn is its AS number, or 0 if it isn't known.
	asn uint32
	// org is the organisation owning it, or "".
	org string
}

// networkOf is the network ip is in, from db, or the zero network if db
// doesn't know (or isn't loaded).
func networkOf(db *database, ip netip.Addr) network {
	var rec asnRecord
	if db == nil || !db.lookup(ip, &rec) || rec.Number == 0 {
		return network{}
	}
	return network{asn: rec.Number, org: rec.Org}
}

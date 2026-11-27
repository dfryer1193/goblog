package stats

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"os"
)

// visitorIDBytes is how many bytes of the HMAC a Visitor ID keeps: 8, so
// a Visitor ID is 16 hex digits.
const visitorIDBytes = 8

// visitorIDFor is the Visitor ID of the client at ip with userAgent under
// salt: the first visitorIDBytes of HMAC-SHA-256(salt, ip NUL userAgent),
// in hex.
func visitorIDFor(salt []byte, ip netip.Addr, userAgent string) string {
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(ip.String()))
	mac.Write([]byte{0})
	mac.Write([]byte(userAgent))
	return hex.EncodeToString(mac.Sum(nil)[:visitorIDBytes])
}

// readSalt reads the salt file at path.
func readSalt(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return b
}

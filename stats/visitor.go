package stats

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"os"
	"sync"
	"time"
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

// saltCheckInterval is how often, at most, the salt file is reread.
const saltCheckInterval = 5 * time.Second

// salt is the daily salt for Visitor IDs, from a file that's replaced at
// 00:00 UTC. It's reread at most every saltCheckInterval, so a new salt
// applies without a restart.
type salt struct {
	path string
	now  func() time.Time

	mu sync.Mutex
	// key is the salt, or nil if there's none.
	key []byte
	// checked is when the file was last read.
	checked time.Time
}

// newSalt reads the salt file at path, which is empty for none.
func newSalt(path string, now func() time.Time) *salt {
	s := &salt{path: path, now: now}
	s.reload(now())
	return s
}

// current is the salt now, or nil if there's none.
func (s *salt) current() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now := s.now(); now.Sub(s.checked) >= saltCheckInterval || now.Before(s.checked) {
		s.reload(now)
	}
	return s.key
}

// reload rereads the salt file at now.
func (s *salt) reload(now time.Time) {
	s.checked = now
	s.key = nil
	if s.path == "" {
		return
	}
	if b, err := os.ReadFile(s.path); err == nil {
		s.key = b
	}
}

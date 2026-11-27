package stats

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

// minSaltBytes is the shortest salt that's used.
const minSaltBytes = 32

// salt is the daily salt for Visitor IDs, from a file that's replaced at
// 00:00 UTC. It's reread at most every saltCheckInterval, so a new salt
// applies without a restart. goblog never makes one up: without a usable
// salt file, there are no Visitor IDs.
type salt struct {
	path   string
	now    func() time.Time
	logger *slog.Logger
	loaded prometheus.Gauge

	mu sync.Mutex
	// key is the salt, or nil if there's none.
	key []byte
	// problem is why there's no salt, or "" if there is one. A warning is
	// logged whenever it changes to a new problem.
	problem string
	// checked is when the file was last read.
	checked time.Time
}

// newSalt reads the salt file at path, which is empty for none. It sets
// loaded to 1 while there's a salt and 0 while there isn't, and logs a
// warning to logger whenever the salt becomes unusable.
func newSalt(path string, now func() time.Time, logger *slog.Logger, loaded prometheus.Gauge) *salt {
	s := &salt{path: path, now: now, logger: logger, loaded: loaded, problem: "not read yet"}
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
	key, problem := s.read()
	s.key = key
	if key != nil {
		s.loaded.Set(1)
		if s.problem != "" {
			s.logger.Info("Loaded the salt for Visitor IDs", "file", s.path)
		}
	} else {
		s.loaded.Set(0)
		if problem != s.problem {
			s.logger.Warn("Page Views have no Visitor ID: "+problem, "file", s.path)
		}
	}
	s.problem = problem
}

// read reads the salt file, and returns the salt, or why there's none. It
// never says what's in the file.
func (s *salt) read() ([]byte, string) {
	if s.path == "" {
		return nil, "the salt file isn't set"
	}
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, "the salt file is missing"
	case err != nil:
		return nil, "the salt file can't be read"
	case len(b) == 0:
		return nil, "the salt file is empty"
	case len(b) < minSaltBytes:
		return nil, fmt.Sprintf("the salt file is shorter than %d bytes", minSaltBytes)
	default:
		return b, ""
	}
}

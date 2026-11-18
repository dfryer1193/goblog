package api

import (
	"fmt"
	"net/http"

	"github.com/dfryer1193/mjolnir/health"
	"github.com/dfryer1193/mjolnir/router"
)

// placeholderPage is what goblog serves until it renders a real site.
const placeholderPage = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>goblog</title></head>
<body><p>goblog is running. There's nothing to read here yet.</p></body>
</html>
`

// NewRouter builds goblog's HTTP handler: mjolnir's router, with its probes
// (/livez, /readyz and /startupz), and goblog's routes. ready backs /readyz.
func NewRouter(ready health.ReadyFunc) (*router.Router, error) {
	r, err := router.New(router.Config{
		Health: health.Config{
			Started: health.AlwaysStarted,
			Ready:   ready,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("building router: %w", err)
	}

	r.Get("/", servePlaceholder)
	return r, nil
}

func servePlaceholder(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(placeholderPage))
}

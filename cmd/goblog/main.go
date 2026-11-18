// Command goblog is an HTTP server, a Deployable. It serves mjolnir's
// probes (/livez, /readyz and /startupz) and drains gracefully on SIGTERM.
package main

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/dfryer1193/mjolnir/health"
	"github.com/dfryer1193/mjolnir/router"
	"github.com/dfryer1193/mjolnir/utils/log"
)

// defaultPort is the port goblog listens on when PORT isn't set.
const defaultPort = "8080"

// drainBudget is the whole shutdown budget: half with readiness at 503, half
// for Shutdown. Keep it under the orchestrator's termination grace period.
const drainBudget = 10 * time.Second

func main() {
	lg := log.GetLogger("goblog")

	r, err := router.New(router.Config{
		Health: health.Config{
			Started: health.AlwaysStarted,
			Ready:   ready,
		},
	})
	if err != nil {
		lg.Error("Failed to build router", "error", err)
		os.Exit(1)
	}

	// TODO: register goblog's routes on r. Put the handlers in
	// github.com/dfryer1193/goblog/api, and have them call github.com/dfryer1193/goblog/service.

	addr := net.JoinHostPort("", port())
	lg.Info("Server starting", "addr", addr)
	if err = r.Run(router.NewServer(addr, r), drainBudget); err != nil {
		lg.Error("Server failed", "error", err)
		os.Exit(1)
	}
	lg.Info("Server stopped")
}

// ready reports whether goblog can serve traffic right now. It is
// consulted on every /readyz request, so it must be cheap.
//
// TODO: check goblog's dependencies (databases, downstream services).
// Until then it reports not-ready, so /readyz answers 503.
func ready(context.Context) bool {
	return false
}

// port is the port to listen on: PORT if set, otherwise defaultPort.
func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return defaultPort
}

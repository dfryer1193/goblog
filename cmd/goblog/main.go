// Command goblog is an HTTP server, a Deployable. It serves mjolnir's
// probes (/livez, /readyz and /startupz) and drains gracefully on SIGTERM.
//
// `goblog build -out dir [-content dir] [-drafts]` instead renders the site
// once from a Content Repository checkout into dir, for previewing.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/dfryer1193/mjolnir/router"
	"github.com/dfryer1193/mjolnir/utils/log"

	"github.com/dfryer1193/goblog/api"
)

// defaultPort is the port goblog listens on when PORT isn't set.
const defaultPort = "8080"

// drainBudget is the whole shutdown budget: half with readiness at 503, half
// for Shutdown. Keep it under the orchestrator's termination grace period.
const drainBudget = 10 * time.Second

func main() {
	if len(os.Args) > 1 && os.Args[1] == "build" {
		if err := runBuild(os.Args[2:], os.Stderr); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "goblog build:", err)
			os.Exit(1)
		}
		return
	}
	serve()
}

// serve is goblog's default mode: the HTTP server.
func serve() {
	lg := log.GetLogger("goblog")

	r, err := api.NewRouter(ready)
	if err != nil {
		lg.Error("Failed to build router", "error", err)
		os.Exit(1)
	}

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
// The placeholder page needs nothing, so goblog is always ready for now.
// Once goblog renders a site, this reports ready only after the first render
// has been swapped in.
func ready(context.Context) bool {
	return true
}

// port is the port to listen on: PORT if set, otherwise defaultPort.
func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return defaultPort
}

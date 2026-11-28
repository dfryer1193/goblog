// Command goblog is an HTTP server, a Deployable. It clones the Content
// Repository, renders the site and serves it, along with mjolnir's probes
// (/livez, /readyz and /startupz), and its Site Statistics metrics on a
// separate port, and drains gracefully on SIGTERM. See
// package app for its environment variables.
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

	"github.com/dfryer1193/goblog/app"
	"github.com/dfryer1193/goblog/stats"
)

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

// serve is goblog's default mode: the HTTP server, publishing the Content
// Repository configured in the environment.
func serve() {
	// mjolnir's JSON log stream defaults to logs/ under the working
	// directory, which may not be writable in the container image. goblog
	// logs to stdout only, so send that stream nowhere.
	if err := log.SetJSONFile(os.DevNull); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "goblog: disabling the JSON log:", err)
	}
	lg := log.GetLogger("goblog")

	cfg, err := app.ConfigFromEnv(os.LookupEnv)
	if err != nil {
		lg.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}
	// The StatsConfig comes from Mimir, live, in serve mode only. Its client
	// runs for the life of the process, and until it has read its first
	// values (up to 5 seconds), the defaults apply.
	cfg.StatsConfig = stats.StartMimirSettings(context.Background(), log.GetLogger("mimir"))
	a, err := app.New(cfg)
	if err != nil {
		lg.Error("Failed to start", "error", err)
		os.Exit(1)
	}

	// Serve probes straight away; readiness turns true once the first sync
	// has rendered the site. Polling keeps it in sync from then on, and
	// retries a failed first sync.
	go a.Run(context.Background())

	// The Site Statistics metrics get their own port, so they're never
	// public through the site's route. If it fails, the site still serves.
	metricsAddr := net.JoinHostPort("", cfg.MetricsPort)
	go func() {
		lg.Info("Metrics server starting", "addr", metricsAddr)
		if serveErr := router.NewServer(metricsAddr, a.MetricsHandler()).ListenAndServe(); serveErr != nil {
			lg.Error("Metrics server failed; still serving the site", "error", serveErr)
		}
	}()

	r := a.Router()
	addr := net.JoinHostPort("", cfg.Port)
	lg.Info("Server starting", "addr", addr)
	if err = r.Run(router.NewServer(addr, r), drainBudget); err != nil {
		lg.Error("Server failed", "error", err)
		os.Exit(1)
	}
	a.Close()
	lg.Info("Server stopped")
}

package stats_test

import (
	"log/slog"
	"testing"

	"github.com/worldtree-industries/mimir/client"

	"github.com/dfryer1193/goblog/stats"
)

// TestStatsConfigItemsDefaultToTheSpec checks the generated Mimir Config's
// Defaults, the way Mimir's own generated tests do. The client sees an empty
// environment, so it runs on Defaults only and reads nothing.
func TestStatsConfigItemsDefaultToTheSpec(t *testing.T) {
	t.Parallel()
	cfg := stats.NewStatsConfig(t.Context(),
		client.WithGetenv(func(string) string { return "" }),
		client.WithLogger(slog.New(slog.DiscardHandler)),
	)

	if !cfg.GetEnabled() {
		t.Error("enabled = false, want true")
	}
	if n := cfg.GetExcludedNetworks().Len(); n != 0 {
		t.Errorf("excluded_networks has %d entries, want none: %v", n, cfg.GetExcludedNetworks().Items())
	}
	if want := "github.com/dfryer1193/goblog/stats.StatsConfig"; stats.StatsConfigName != want {
		t.Errorf("StatsConfigName = %q, want %q", stats.StatsConfigName, want)
	}
}

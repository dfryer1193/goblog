package stats

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/dfryer1193/mjolnir/utils/set"
	"github.com/worldtree-industries/mimir/client"
)

// mimirSettings are the StatsConfig from Mimir: the generated StatsConfig,
// behind Settings. Until its client has started, they're the Defaults.
type mimirSettings struct {
	// cfg is the generated StatsConfig, or nil until its client has started.
	cfg atomic.Pointer[StatsConfig]

	// mu guards the cached excluded_networks.
	mu sync.Mutex
	// excludedSet is the excluded_networks value excluded was made from. A
	// new value is a new Set: the client replaces it, never changes it.
	excludedSet set.Set[string]
	// excluded is excludedSet's entries, sorted.
	excluded []string
}

// StartMimirSettings starts Mimir's client for the StatsConfig, logging to
// logger, and returns the settings it keeps current until ctx is done.
// Only serve mode calls it. It doesn't wait for the client, which can take
// up to 5 seconds to read its first values: until then, the settings are
// the Defaults.
//
// The client is the first in the process, so its options set up the
// process's Mimir Runtime: the Namespace is Deployable goblog (the main
// package's name, or MIMIR_DEPLOYABLE) and the Environment in
// MIMIR_ENVIRONMENT, and Snapshots are read from object storage with the
// standard AWS environment variables. Without those, it runs on the
// Defaults.
func StartMimirSettings(ctx context.Context, logger *slog.Logger) Settings {
	s := &mimirSettings{}
	go func() {
		s.cfg.Store(NewStatsConfig(ctx, client.WithLogger(logger)))
	}()
	return s
}

// Enabled is the enabled Item.
func (s *mimirSettings) Enabled() bool {
	cfg := s.cfg.Load()
	if cfg == nil {
		return Defaults().Enabled()
	}
	return cfg.GetEnabled()
}

// ExcludedNetworks is the excluded_networks Item, sorted, so an unchanged
// value is always the same list.
func (s *mimirSettings) ExcludedNetworks() []string {
	cfg := s.cfg.Load()
	if cfg == nil {
		return Defaults().ExcludedNetworks()
	}
	current := cfg.GetExcludedNetworks()
	s.mu.Lock()
	defer s.mu.Unlock()
	if current != s.excludedSet {
		s.excludedSet = current
		s.excluded = slices.Sorted(slices.Values(current.Items()))
	}
	return s.excluded
}

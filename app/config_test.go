package app_test

import (
	"testing"
	"time"

	"github.com/dfryer1193/goblog/app"
)

// configFrom is app.ConfigFromEnv over env.
func configFrom(env map[string]string) (app.Config, error) {
	env["GOBLOG_CONTENT_URL"] = "file:///nowhere"
	return app.ConfigFromEnv(func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	})
}

func TestPollIntervalComesFromTheEnvironment(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]time.Duration{"": time.Minute, "30s": 30 * time.Second, "5m": 5 * time.Minute} {
		cfg, err := configFrom(map[string]string{"GOBLOG_POLL_INTERVAL": value})
		if err != nil {
			t.Fatalf("GOBLOG_POLL_INTERVAL=%q: %v", value, err)
		}
		if cfg.PollInterval != want {
			t.Errorf("GOBLOG_POLL_INTERVAL=%q: PollInterval = %v, want %v", value, cfg.PollInterval, want)
		}
	}
}

func TestInvalidPollIntervalIsAnError(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"soon", "60", "0s", "-1m"} {
		if _, err := configFrom(map[string]string{"GOBLOG_POLL_INTERVAL": value}); err == nil {
			t.Errorf("GOBLOG_POLL_INTERVAL=%q: no error", value)
		}
	}
}

func TestWebhookSecretComesFromTheEnvironment(t *testing.T) {
	t.Parallel()
	cfg, err := configFrom(map[string]string{"GOBLOG_WEBHOOK_SECRET": "s3cret"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebhookSecret != "s3cret" {
		t.Errorf("WebhookSecret = %q, want s3cret", cfg.WebhookSecret)
	}
}

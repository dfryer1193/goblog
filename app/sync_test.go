package app_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/dfryer1193/goblog/service"
)

// pushBody is a webhook push event's body. goblog doesn't read it, but the
// signature covers it.
const pushBody = `{"ref":"refs/heads/trunk"}`

func TestSignedPushWebhookSyncs(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	repo.commit(t, map[string]string{"posts/second.md": post("Second", "2026-03-05")})

	resp := g.webhook(t, "push", pushBody, sign(webhookSecret, pushBody))
	g.app.WaitForSyncs()

	assertStatus(t, "POST /webhook", resp, http.StatusAccepted)
	assertStatus(t, "/posts/second/", g.get(t, "/posts/second/"), http.StatusOK)
}

func TestWebhookWithoutAValidSignatureIsRejectedAndDoesntSync(t *testing.T) {
	t.Parallel()
	for name, signature := range map[string]string{
		"missing":        "",
		"wrong secret":   sign("not-the-secret", pushBody),
		"other body":     sign(webhookSecret, pushBody+" "),
		"not hex":        "sha256=zz",
		"no prefix":      sign(webhookSecret, pushBody)[len("sha256="):],
		"sha1 signature": "sha1=0123456789abcdef0123456789abcdef01234567",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := newContentRepo(t)
			repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
			g := startGoblog(t, repo, nil)
			g.sync(t)
			repo.commit(t, map[string]string{"posts/second.md": post("Second", "2026-03-05")})

			resp := g.webhook(t, "push", pushBody, signature)
			g.app.WaitForSyncs()

			assertStatus(t, "POST /webhook", resp, http.StatusUnauthorized)
			assertStatus(t, "/posts/second/", g.get(t, "/posts/second/"), http.StatusNotFound)
		})
	}
}

func TestWebhookAcceptsPing(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	body := `{"zen":"Keep it logically awesome."}`

	resp := g.webhook(t, "ping", body, sign(webhookSecret, body))

	if resp.status < 200 || resp.status > 299 {
		t.Fatalf("POST /webhook ping status = %d, want 2xx; body:\n%s", resp.status, resp.body)
	}
}

func TestWebhookIsDisabledWithoutASecret(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, map[string]string{"GOBLOG_WEBHOOK_SECRET": ""})
	g.sync(t)
	repo.commit(t, map[string]string{"posts/second.md": post("Second", "2026-03-05")})

	resp := g.webhook(t, "push", pushBody, sign("", pushBody))
	g.app.WaitForSyncs()

	assertStatus(t, "POST /webhook", resp, http.StatusNotFound)
	assertStatus(t, "/posts/second/", g.get(t, "/posts/second/"), http.StatusNotFound)
}

func TestPollPublishesANewCommit(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	repo.commit(t, map[string]string{"posts/second.md": post("Second", "2026-03-05")})

	g.poll(t, service.PollSynced)

	assertStatus(t, "/posts/second/", g.get(t, "/posts/second/"), http.StatusOK)
}

func TestPollWithoutANewCommitDoesNothing(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)

	g.poll(t, service.PollUnchanged)

	assertStatus(t, "/posts/hello/", g.get(t, "/posts/hello/"), http.StatusOK)
}

func TestPollRetriesAFailedFirstSync(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	g := startGoblog(t, repo, nil)
	g.syncFails(t) // the Content Repository has no commits yet
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})

	g.poll(t, service.PollSynced)

	assertStatus(t, "/readyz", g.get(t, "/readyz"), http.StatusOK)
	assertStatus(t, "/posts/hello/", g.get(t, "/posts/hello/"), http.StatusOK)
}

func TestPollPublishesAScheduledPostWhenItsDayArrivesInTheSitesTimezone(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{
		// Los Angeles is on UTC-7 from 2026-03-08.
		"site.yaml":          "title: Test\ntimezone: America/Los_Angeles\n",
		"posts/hello.md":     post("Hello", "2026-03-01"),
		"posts/scheduled.md": post("Scheduled", "2026-03-11"),
	})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	assertStatus(t, "/posts/scheduled/", g.get(t, "/posts/scheduled/"), http.StatusNotFound)

	g.clock.set(time.Date(2026, time.March, 11, 6, 59, 0, 0, time.UTC)) // 23:59 on the 10th in Los Angeles
	g.poll(t, service.PollUnchanged)
	assertStatus(t, "/posts/scheduled/", g.get(t, "/posts/scheduled/"), http.StatusNotFound)

	g.clock.set(time.Date(2026, time.March, 11, 7, 0, 0, 0, time.UTC)) // midnight on the 11th in Los Angeles
	g.poll(t, service.PollRerendered)
	assertStatus(t, "/posts/scheduled/", g.get(t, "/posts/scheduled/"), http.StatusOK)

	g.poll(t, service.PollUnchanged)
}

func TestPollPublishesAScheduledPostEvenIfTheRemoteIsUnreachable(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/scheduled.md": post("Scheduled", "2026-03-11")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	repo.remove(t)

	g.clock.set(time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC))
	result, err := g.app.Poll(t.Context())

	if err == nil {
		t.Error("poll with the remote gone succeeded, want the failed check reported")
	}
	if result != service.PollRerendered {
		t.Errorf("poll = %v, want %v", result, service.PollRerendered)
	}
	assertStatus(t, "/posts/scheduled/", g.get(t, "/posts/scheduled/"), http.StatusOK)
}

func TestPollKeepsTheCurrentSiteWhenTheNewCommitCantBeRendered(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	repo.commit(t, map[string]string{
		"site.yaml":       "title: Test\ntimezone: Not/A_Timezone\n",
		"posts/second.md": post("Second", "2026-03-05"),
	})

	result, err := g.app.Poll(t.Context())

	if err == nil || result != service.PollSynced {
		t.Fatalf("poll = %v, %v; want %v and the render's error", result, err, service.PollSynced)
	}
	assertStatus(t, "/readyz", g.get(t, "/readyz"), http.StatusOK)
	assertStatus(t, "/posts/hello/", g.get(t, "/posts/hello/"), http.StatusOK)
	assertStatus(t, "/posts/second/", g.get(t, "/posts/second/"), http.StatusNotFound)
	// The broken commit isn't fetched and rendered again on every poll.
	g.poll(t, service.PollUnchanged)
}

// Run with -race: the clone isn't safe for concurrent use, so overlapping
// syncs or renders show up as data races.
func TestConcurrentTriggersDontOverlap(t *testing.T) {
	t.Parallel()
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	g := startGoblog(t, repo, nil)
	g.sync(t)
	repo.commit(t, map[string]string{
		"posts/second.md":    post("Second", "2026-03-05"),
		"posts/scheduled.md": post("Scheduled", "2026-03-11"),
	})

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if resp := g.webhook(
				t,
				"push",
				pushBody,
				sign(webhookSecret, pushBody),
			); resp.status != http.StatusAccepted {
				t.Errorf("POST /webhook status = %d, want %d", resp.status, http.StatusAccepted)
			}
		})
		wg.Go(func() {
			if _, err := g.app.Poll(t.Context()); err != nil {
				t.Errorf("poll: %v", err)
			}
		})
		wg.Go(func() {
			if err := g.app.Sync(t.Context()); err != nil {
				t.Errorf("sync: %v", err)
			}
		})
		wg.Go(func() { g.clock.advance(3 * time.Hour) })
	}
	wg.Wait()
	g.app.WaitForSyncs()

	assertStatus(t, "/posts/second/", g.get(t, "/posts/second/"), http.StatusOK)
	// The clock ends at 03:00 on the 11th, so the last render was on the
	// 10th or 11th; on the 12th a poll renders again.
	g.clock.set(time.Date(2026, time.March, 12, 12, 0, 0, 0, time.UTC))
	g.poll(t, service.PollRerendered)
	assertStatus(t, "/posts/scheduled/", g.get(t, "/posts/scheduled/"), http.StatusOK)
}

package app_test

import (
	"net/http"
	"testing"
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

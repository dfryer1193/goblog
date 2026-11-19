package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
)

// WebhookPath is where GitHub delivers webhook events. "webhook" is a
// reserved Page name, so no Page can take this path.
const WebhookPath = "/webhook"

// maxWebhookBody bounds the webhook request body. It's GitHub's own payload
// cap. The body is only hashed, never held in memory.
const maxWebhookBody = 25 << 20

// signaturePrefix starts GitHub's X-Hub-Signature-256 header value.
const signaturePrefix = "sha256="

// GitHub webhook event names, from the X-GitHub-Event header.
const (
	eventPing = "ping"
	eventPush = "push"
)

// Webhook configures the webhook endpoint.
type Webhook struct {
	// Secret is the webhook's shared secret, for checking GitHub's HMAC
	// signature. Empty disables the endpoint: its path is then answered
	// like any path the site doesn't have.
	Secret string
	// Trigger asks for a sync in the background. It must return quickly.
	Trigger func()
}

// webhookHandler receives GitHub webhook events: a push with a valid
// signature triggers a sync and gets 202; a ping gets 204. A missing or
// invalid signature gets 401 and triggers nothing. Methods other than POST
// get 405. While the endpoint is disabled, every request, whatever its
// method, is answered like a path the site doesn't have.
type webhookHandler struct {
	hook  Webhook
	files fileHandler
}

func (h webhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.hook.Secret == "" {
		h.files.serveMissing(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !h.validSignature(r) {
		http.Error(w, "invalid or missing X-Hub-Signature-256", http.StatusUnauthorized)
		return
	}
	switch r.Header.Get("X-Github-Event") {
	case eventPush:
		h.hook.Trigger()
		w.WriteHeader(http.StatusAccepted)
	case eventPing:
		w.WriteHeader(http.StatusNoContent)
	default:
		// Not an event goblog acts on; accept it so GitHub doesn't report a
		// failed delivery.
		w.WriteHeader(http.StatusNoContent)
	}
}

// validSignature reports whether r's X-Hub-Signature-256 header is the HMAC
// of its body under the secret. It reads the body.
func (h webhookHandler) validSignature(r *http.Request) bool {
	got, ok := strings.CutPrefix(r.Header.Get("X-Hub-Signature-256"), signaturePrefix)
	if !ok {
		return false
	}
	sig, err := hex.DecodeString(got)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.hook.Secret))
	n, err := io.Copy(mac, io.LimitReader(r.Body, maxWebhookBody+1))
	if err != nil || n > maxWebhookBody {
		return false
	}
	return hmac.Equal(sig, mac.Sum(nil))
}

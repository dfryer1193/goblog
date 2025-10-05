package http

import (
	"net/http"
	"os"

	"github.com/dfryer1193/goblog/blog/application"
	"github.com/go-chi/chi/v5"
	"github.com/google/go-github/v75/github"
)

const (
	repoName = "dfryer1193/blog"
)

type WebhookHandler struct {
	webhookSecret []byte
	postService   *application.PostService
}

func NewWebhookHandler(postService *application.PostService) *WebhookHandler {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		panic("WEBHOOK_SECRET is not set")
	}

	return &WebhookHandler{
		webhookSecret: []byte(secret),
		postService:   postService,
	}
}

func (h *WebhookHandler) RegisterRoutes(r chi.Router) {
	r.Post("/webhook/git", h.HandleGitWebhook)
}

func (h *WebhookHandler) HandleGitWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, h.webhookSecret)
	if err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		http.Error(w, "Invalid event", http.StatusBadRequest)
		return
	}

	switch evt := event.(type) {
	case *github.PushEvent:
		err = h.postService.HandlePushEvent(evt)
	}
	if err != nil {
		http.Error(w, "Error handling event", http.StatusInternalServerError)
		return
	}

	// Handle the event
	w.WriteHeader(http.StatusNoContent)
}



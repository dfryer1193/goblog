package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dfryer1193/goblog/blog/application"
	"github.com/dfryer1193/goblog/blog/persistence"
	gh "github.com/dfryer1193/goblog/shared/github"

	"github.com/dfryer1193/goblog/shared/db"
	"github.com/google/go-github/v75/github"
	"github.com/dfryer1193/mjolnir/router"
	"github.com/rs/zerolog/log"
)

const (
	port            = 8080
	shutdownTimeout = 5 * time.Second
	// TODO: Load from config
	ghOwner = "dfryer1193"
	ghRepo  = "goblog-posts"
)

// TODO: implement a real markdown renderer
 type placeholderMarkdownRenderer struct{}

func (p placeholderMarkdownRenderer) Render(markdown string) (string, error) {
	return "<p>" + markdown + "</p>", nil
}

func main() {
	// Initialize dependencies
	dbConn, err := db.GetConnection(context.Background(), "blog.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer dbConn.Close()

	// TODO: set up authenticated client
	ghClient := github.NewClient(nil)
	sourceRepo := gh.NewGithubSourceRepository(ghClient, ghOwner, ghRepo)

	mainBranchName, err := sourceRepo.GetDefaultBranchName(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get default branch name")
	}

	postRepo := persistence.NewPostRepository(dbConn)
	markdownRenderer := placeholderMarkdownRenderer{}

	postService := application.NewPostService(postRepo, sourceRepo, markdownRenderer, mainBranchName)
	defer func() {
		if err := postService.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to gracefully close post service")
		}
	}()

	r := router.New()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	go func() {
		log.Info().Msg("Starting server on port :" + fmt.Sprint(port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info().Msg("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to shutdown server")
	}

	log.Info().Msg("Server stopped")
}

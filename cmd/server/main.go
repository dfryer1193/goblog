package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/dfryer1193/goblog/blog/application"
	"github.com/dfryer1193/goblog/blog/persistence"
	"github.com/dfryer1193/goblog/shared/db/sqlite"
	ghrepo "github.com/dfryer1193/goblog/shared/github"

	"github.com/dfryer1193/mjolnir/router"

	"github.com/google/go-github/v75/github"
	"github.com/rs/zerolog/log"
)

const (
	port            = 8080
	shutdownTimeout = 5 * time.Second
	repo            = "https://github.com/dfryer1193/blog"
	authTokenEnv    = "GITHUB_AUTH_TOKEN"
	mainBranch      = "main"
	postDir         = "/posts"
)

func main() {
	authToken := os.Getenv(authTokenEnv)
	if authToken == "" {
		log.Fatal().Msgf("Environment variable %s is not set", authTokenEnv)
	}

	dbClient := sqlite.NewSQLiteDB(sqlite.NewSQLiteConfig())
	defer dbClient.Close()

	// Get the underlying sql.DB instance
	db := dbClient.DB()
	postRepo := persistence.NewPostRepository(db)
	imageRepo := persistence.NewImageRepository(db)

	// Create GitHub client and source repository
	ghClient := github.NewClient(nil).WithAuthToken(authToken)
	parts := strings.Split(strings.TrimPrefix(repo, "https://github.com/"), "/")
	if len(parts) != 2 {
		log.Fatal().Msgf("Invalid repository URL: %s", repo)
	}
	owner, gitRepo := parts[0], parts[1]
	sourceRepo := ghrepo.NewGithubSourceRepository(ghClient, owner, gitRepo)

	// Create markdown renderer
	markdownRenderer := application.NewMarkdownRenderer()

	// Get main branch name
	mainBranchName := mainBranch
	defaultBranch, err := sourceRepo.GetDefaultBranchName(context.Background())
	if err == nil {
		mainBranchName = defaultBranch
	}

	postService := application.NewPostService(postRepo, imageRepo, sourceRepo, markdownRenderer, mainBranchName)
	defer postService.Close()

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

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dfryer1193/goblog/blog/application"
	"github.com/dfryer1193/goblog/blog/persistence"
	"github.com/dfryer1193/goblog/shared/db/sqlite"

	"github.com/dfryer1193/mjolnir/router"

	"github.com/rs/zerolog/log"
)

const (
	port            = 8080
	shutdownTimeout = 5 * time.Second
	repo            = "https://github.com/dfryer1193/blog"
	authTokenEnv    = "GITHUB_AUTH_TOKEN"
	postDir         = "/posts"
)

func main() {
	authToken := os.Getenv(authTokenEnv)
	if authToken == "" {
		log.Fatal().Msgf("Environment variable %s is not set", authTokenEnv)
	}

	dbClient := sqlite.NewSQLiteDB(sqlite.NewSQLiteConfig())
	defer dbClient.Close()

	// TODO: Build database client as an sql.DB
	var db *sql.DB
	postRepo := persistence.NewPostRepository()
	postService := application.NewPostService(postRepo, dbClient)
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

func ensurePostDir(path string) error {
	fileInfo, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	if !fileInfo.IsDir() {
		return errors.New("postDir is not a directory")
	}

	return nil
}

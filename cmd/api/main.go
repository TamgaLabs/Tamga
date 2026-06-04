package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TamgaLabs/Tamga/internal/api"
	"github.com/TamgaLabs/Tamga/internal/auth"
	"github.com/TamgaLabs/Tamga/internal/config"
	"github.com/TamgaLabs/Tamga/internal/database"
	"github.com/TamgaLabs/Tamga/internal/deployments"
	dockerclient "github.com/TamgaLabs/Tamga/internal/docker"
	"github.com/TamgaLabs/Tamga/internal/domain"
	"github.com/TamgaLabs/Tamga/internal/envvar"
	"github.com/TamgaLabs/Tamga/internal/git"
	"github.com/TamgaLabs/Tamga/internal/logs"
	"github.com/TamgaLabs/Tamga/internal/project"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	docker, err := dockerclient.NewClient()
	if err != nil {
		log.Fatalf("failed to create docker client: %v", err)
	}
	defer docker.Close()

	queries := database.New(db)
	deploySvc := deployments.NewService(queries, docker, "/tmp/tamga-builds", "tamga")
	logStreamer := logs.NewStreamer(queries, docker)

	handlers := &api.Handlers{
		Auth:       auth.NewHandler(db, cfg.JWTSecret),
		Project:    project.NewHandler(db),
		Domain:     domain.NewHandler(db),
		EnvVar:     envvar.NewHandler(db),
		Git:        git.NewHandler(db),
		Deployment: deployments.NewHandler(db, deploySvc),
		Logs:       logs.NewHandler(db, logStreamer),
	}

	router := api.SetupRouter(handlers, cfg.JWTSecret)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server starting on port %d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
}

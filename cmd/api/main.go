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
	"github.com/TamgaLabs/Tamga/internal/feature/auth"
	"github.com/TamgaLabs/Tamga/internal/feature/project"
	"github.com/TamgaLabs/Tamga/internal/config"
	"github.com/TamgaLabs/Tamga/internal/db"
)

func main() {
	cfg := config.Load()

	database, err := db.Connect(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("failed to get underlying DB: %v", err)
	}
	defer sqlDB.Close()

	if err := database.AutoMigrate(
		&auth.User{},
		&project.Project{},
	); err != nil {
		log.Fatalf("failed to initialize schema: %v", err)
	}
	log.Println("database schema initialized")

	authRepo := auth.NewRepository(database)
	projectRepo := project.NewRepository(database)

	handlers := &api.Handlers{
		Auth:    auth.NewHandler(authRepo, cfg.JWTSecret),
		Project: project.NewHandler(projectRepo),
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

package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/handler"
	caddyrepo "github.com/TamgaLabs/Tamga/backend/internal/repository/caddy"
	dockerrepo "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/router"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func main() {
	cfg := config.Load()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations completed")

	if err := db.EnsureTables(); err != nil {
		slog.Error("failed to ensure tables", "error", err)
		os.Exit(1)
	}

	authService := service.NewAuthService(db, cfg)

	if err := authService.AutoSetup(); err != nil {
		slog.Warn("auto setup", "error", err)
	} else {
		slog.Info("admin user ready")
	}

	dockerClient, err := dockerrepo.New()
	if err != nil {
		slog.Warn("docker client not available, deploy disabled", "error", err)
	}
	caddyClient := caddyrepo.New(cfg.CaddyAdminURL)

	agentProviderService := service.NewAgentProviderService(db)
	apiKeyService := service.NewApiKeyService(db, cfg.JWTSecret)
	whitelistService := service.NewWhitelistService(db)
	projectService := service.NewProjectService(db, dockerClient, caddyClient, cfg)
	agentService := service.NewAgentService(db, dockerClient, cfg, agentProviderService, apiKeyService, whitelistService)

	if err := setupCaddyRoutes(caddyClient, cfg); err != nil {
		slog.Warn("caddy route setup", "error", err)
	} else {
		// Reconcile existing project routes after LoadConfig
		// (LoadConfig replaces Caddy's entire config, so we need to restore per-project routes)
		reconcileProjectRoutes(projectService, dockerClient, caddyClient)
	}

	systemHandler := handler.NewSystemHandler()
	authHandler := handler.NewAuthHandler(authService)
	projectHandler := handler.NewProjectHandler(projectService)
	terminalHandler := handler.NewTerminalHandler(agentService)
	codeHandler := handler.NewCodeHandler(projectService, cfg)
	agentProviderHandler := handler.NewAgentProviderHandler(agentProviderService)
	apiKeyHandler := handler.NewApiKeyHandler(apiKeyService)
	whitelistHandler := handler.NewWhitelistHandler(whitelistService)
	authMiddleware := handler.AuthMiddleware(authService)

	var containerHandler *handler.ContainerHandler
	if dockerClient != nil {
		containerHandler = handler.NewContainerHandler(dockerClient)
	} else {
		containerHandler = handler.NewContainerHandler(nil)
	}

	r := router.New(authHandler, systemHandler, projectHandler, terminalHandler, containerHandler, codeHandler, agentProviderHandler, apiKeyHandler, whitelistHandler, authMiddleware)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	slog.Info("server stopped")
}

const caddyfilePath = "/Caddyfile"

func setupCaddyRoutes(c *caddyrepo.Client, cfg config.Config) error {
	var buf bytes.Buffer

	buf.WriteString("{\n")
	buf.WriteString("\tadmin :2019\n")
	if cfg.CaddyAutoSSL {
		buf.WriteString(fmt.Sprintf("\temail %s\n", cfg.CaddyEmail))
	} else {
		buf.WriteString("\tauto_https off\n")
	}
	buf.WriteString("}\n\n")

	if !cfg.CaddyAutoSSL {
		buf.WriteString(fmt.Sprintf("%s:80 {\n", cfg.UIDomain))
	} else {
		buf.WriteString(fmt.Sprintf("%s {\n", cfg.UIDomain))
	}
	buf.WriteString("\t@api path /api/*\n")
	buf.WriteString("\thandle @api {\n")
	buf.WriteString("\t\treverse_proxy backend:8080\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\thandle {\n")
	buf.WriteString("\t\treverse_proxy frontend:3000\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n")

	caddyfileContent := buf.Bytes()

	// Write Caddyfile to disk for reference/debugging
	if err := os.WriteFile(caddyfilePath, caddyfileContent, 0644); err != nil {
		return fmt.Errorf("write caddyfile: %w", err)
	}
	slog.Info("caddyfile written", "path", caddyfilePath)

	// Load config via Caddy admin API (POST /load with Caddyfile format)
	if err := c.LoadConfig(caddyfileContent); err != nil {
		slog.Warn("caddy config load failed", "error", err)
		return err
	}
	slog.Info("caddy config loaded successfully")

	return nil
}

// reconcileProjectRoutes restores all deployed project routes after LoadConfig
// replaces Caddy's entire configuration. This ensures that backend restarts
// don't wipe out live routing for deployed projects.
func reconcileProjectRoutes(ps *service.ProjectService, dc *dockerrepo.Client, cc *caddyrepo.Client) {
	if ps == nil || dc == nil {
		return
	}

	ctx := context.Background()
	projects, err := ps.List(ctx)
	if err != nil {
		slog.Warn("reconcile routes: list projects failed", "error", err)
		return
	}

	for _, p := range projects {
		// Only restore routes for deployed projects with active containers
		if p.Status != domain.ProjectStatusRunning || p.ContainerID == "" || p.Domain == "" {
			continue
		}

		// Get container port (default to 80 if unavailable)
		port, err := dc.GetContainerPort(ctx, p.ContainerID)
		if err != nil {
			port = "80"
		}

		// Reconstruct the exact upstream string that would have been used
		upstream := fmt.Sprintf("project-%d:%s", p.ID, port)

		// Re-add the route (non-fatal if it fails)
		if err := cc.AddRoute(p.Domain, upstream); err != nil {
			slog.Warn("reconcile project route", "project_id", p.ID, "domain", p.Domain, "error", err)
		} else {
			slog.Info("reconciled project route", "project_id", p.ID, "domain", p.Domain, "upstream", upstream)
		}
	}
}

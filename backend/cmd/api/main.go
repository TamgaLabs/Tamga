package main

import (
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
	dockerrepo "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	traefikrepo "github.com/TamgaLabs/Tamga/backend/internal/repository/traefik"
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
	traefikClient := traefikrepo.New(cfg.TraefikDynamicDir)
	if err := traefikClient.EnsureDir(); err != nil {
		slog.Warn("ensure traefik dynamic dir", "error", err)
	}

	whitelistService := service.NewWhitelistService(db)
	egressService := service.NewEgressService(db)
	resourceLimitService := service.NewResourceLimitService(db)
	idleTimeoutService := service.NewIdleTimeoutService(db)
	gitCredentialService := service.NewGitCredentialService(db, cfg.JWTSecret)
	projectService := service.NewProjectService(db, dockerClient, traefikClient, cfg, gitCredentialService)
	agentService := service.NewAgentService(db, dockerClient, cfg, whitelistService, egressService, resourceLimitService, gitCredentialService, idleTimeoutService)

	// Re-write every running project's route file on boot. Unlike Caddy's
	// admin API (whose config lived entirely in the Caddy process's
	// memory, wiped and reloaded on every backend restart), Traefik's
	// file-provider routes persist on disk across backend restarts by
	// themselves - this isn't a reconcile-after-wipe like Caddy needed,
	// it's a defensive re-write in case the dynamic dir was ever cleared
	// or a file is missing/stale (e.g. the container's port changed since
	// the file was last written), so it can't drift silently.
	reconcileProjectRoutes(projectService, dockerClient, traefikClient)

	systemHandler := handler.NewSystemHandler()
	authHandler := handler.NewAuthHandler(authService)
	projectHandler := handler.NewProjectHandler(projectService)
	terminalHandler := handler.NewTerminalHandler(agentService)
	codeHandler := handler.NewCodeHandler(projectService, cfg)
	whitelistHandler := handler.NewWhitelistHandler(whitelistService)
	egressHandler := handler.NewEgressHandler(egressService)
	resourceLimitHandler := handler.NewResourceLimitHandler(resourceLimitService)
	idleTimeoutHandler := handler.NewIdleTimeoutHandler(idleTimeoutService)
	gitCredentialHandler := handler.NewGitCredentialHandler(gitCredentialService)
	authMiddleware := handler.AuthMiddleware(authService)

	var containerHandler *handler.ContainerHandler
	if dockerClient != nil {
		containerHandler = handler.NewContainerHandler(dockerClient)
	} else {
		containerHandler = handler.NewContainerHandler(nil)
	}

	r := router.New(authHandler, systemHandler, projectHandler, terminalHandler, containerHandler, codeHandler, whitelistHandler, egressHandler, resourceLimitHandler, idleTimeoutHandler, gitCredentialHandler, authMiddleware)

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

// reconcileProjectRoutes re-writes the dynamic-config route file for every
// currently-running project on backend startup. This is not a Caddy-style
// "restore everything after the whole config got wiped" reconcile - each
// project's Traefik route file is independent and Traefik's file-provider
// watcher (providers.file.watch, traefik/traefik.yml) hot-reloads on
// change with no coordination needed between files, so a backend restart
// alone can never wipe another project's route. Rewriting on boot instead
// guards against drift: the dynamic dir is a bind mount that could be
// cleared/tampered with outside the backend's control, and a project's
// upstream port is re-derived fresh from the live container rather than
// trusted from whatever was last written, so a stale/missing file self-heals
// on the next restart.
func reconcileProjectRoutes(ps *service.ProjectService, dc *dockerrepo.Client, tc *traefikrepo.Client) {
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

		// Re-write the route (non-fatal if it fails)
		if err := tc.AddRoute(p.ID, p.Domain, upstream); err != nil {
			slog.Warn("reconcile project route", "project_id", p.ID, "domain", p.Domain, "error", err)
		} else {
			slog.Info("reconciled project route", "project_id", p.ID, "domain", p.Domain, "upstream", upstream)
		}
	}
}

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
	// Starts its own background scrape loop (FEAT-031).
	service.NewMetricsScraperService(db, cfg.TraefikMetricsURL, cfg.TraefikMetricsPeriod)
	// Starts its own background rollup+retention sweep (FEAT-032) - see
	// service.MetricsRollupService's doc for the minute->hour->day policy.
	// Its return value isn't captured beyond construction either; only the
	// query side (metricsQueryService below) is a handler dependency.
	service.NewMetricsRollupService(db, service.DefaultMetricsRollupInterval)
	metricsQueryService := service.NewMetricsQueryService(db)
	topologyService := service.NewTopologyService(dockerClient)

	// Re-write every running project's route file on boot. Unlike Caddy's
	// admin API (whose config lived entirely in the Caddy process's
	// memory, wiped and reloaded on every backend restart), Traefik's
	// file-provider routes persist on disk across backend restarts by
	// themselves - this isn't a reconcile-after-wipe like Caddy needed,
	// it's a defensive re-write in case the dynamic dir was ever cleared
	// or a file is missing/stale (e.g. the container's port changed since
	// the file was last written), so it can't drift silently. Also
	// re-attaches Traefik to every reconciled project's network
	// (FEAT-028's per-project-network reachability design), in case that
	// attachment itself was ever lost.
	projectService.ReconcileRoutes(context.Background())

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
	metricsHandler := handler.NewMetricsHandler(metricsQueryService)
	topologyHandler := handler.NewTopologyHandler(topologyService)
	authMiddleware := handler.AuthMiddleware(authService)

	var containerHandler *handler.ContainerHandler
	if dockerClient != nil {
		containerHandler = handler.NewContainerHandler(dockerClient)
	} else {
		containerHandler = handler.NewContainerHandler(nil)
	}

	r := router.New(authHandler, systemHandler, projectHandler, terminalHandler, containerHandler, codeHandler, whitelistHandler, egressHandler, resourceLimitHandler, idleTimeoutHandler, gitCredentialHandler, metricsHandler, topologyHandler, authMiddleware)

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

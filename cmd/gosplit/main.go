// Command gosplit is the GoSplit server: a single static binary serving the
// full app (server-rendered HTML + embedded assets) over chi, backed by SQLite
// (default) or PostgreSQL.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hafio/gosplit/internal/auth"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/httpapp"
	"github.com/hafio/gosplit/internal/mail"
	"github.com/hafio/gosplit/internal/scheduler"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/store"
	"github.com/hafio/gosplit/internal/web"
)

// version is the release tag, stamped at build time by the dev scripts and the
// Dockerfile via -ldflags "-X main.version=...". An unstamped binary says "dev".
var version = "dev"

// wantsVersion reports whether the CLI args ask for the version banner.
func wantsVersion(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[1] {
	case "version", "-version", "--version":
		return true
	}
	return false
}

func main() {
	if wantsVersion(os.Args) {
		fmt.Println(version)
		return
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// parseLogLevel maps a LOG_LEVEL string to an slog level, defaulting to Info.
func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Re-configure the logger at the requested level now that config is loaded.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)})))
	slog.Info("starting", "version", version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	st, err := store.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	slog.Info("database ready", "engine", cfg.Engine)

	mailer := mail.New(cfg)
	svc := service.New(st, mailer, cfg)
	am := auth.NewManager(st, cfg.SecureCookies)

	renderer, err := web.NewRenderer()
	if err != nil {
		return err
	}

	app := httpapp.New(cfg, st, svc, am, renderer)

	if cfg.Scheduler {
		host, _ := os.Hostname()
		sched := scheduler.New(svc, host+"-"+time.Now().Format("150405"))
		go sched.Run(ctx)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "base_url", cfg.BaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// Package main is the entry point for the contacthub server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/config"
	"github.com/sdobberstein/contacthub/internal/handler"
	"github.com/sdobberstein/contacthub/internal/middleware"
	"github.com/sdobberstein/contacthub/internal/migrations"
	"github.com/sdobberstein/contacthub/internal/store/sqlite"
	"github.com/sdobberstein/contacthub/internal/wellknown"
)

const version = "0.1.0-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var configPath string
	var showVersion bool

	flag.StringVar(&configPath, "config", "", "path to config.yaml (optional)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("contacthub %s\n", version)
		return nil
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := newLogger(cfg.Log)

	db, err := sqlite.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close() //nolint:errcheck // best-effort on shutdown

	if err := migrations.Run(context.Background(), db.DB()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if err := bootstrapAdmin(context.Background(), db, cfg.Admin, logger); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}

	tmpl, err := template.ParseGlob("templates/**/*.html")
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	authProvider := local.New(db)

	// Register WebDAV HTTP methods not in chi's default set.
	for _, m := range []string{"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "REPORT", "ACL", "LOCK", "UNLOCK"} {
		chi.RegisterMethod(m)
	}

	r := chi.NewRouter()
	r.Use(middleware.ProxyHeaders(cfg.Server.TrustedProxies, cfg.Server.BaseURL, cfg.Server.PathPrefix))
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestLogger(logger))

	r.Get("/healthz", handleHealthz(db))

	// RFC 6764 §5: well-known redirect — no auth required, fixed context path.
	r.HandleFunc("/.well-known/carddav", wellknown.Handler)

	// First-run setup (only accessible when no users exist).
	r.With(middleware.SetupGuard(db)).
		HandleFunc("/setup", handler.SetupHandler(db, tmpl))

	// Auth routes — rate-limited login.
	r.Route("/auth", func(r chi.Router) {
		r.With(middleware.LoginRateLimiter(cfg.Auth.RateLimit)).
			HandleFunc("/login", handler.LoginHandler(db, authProvider, cfg.Auth, tmpl))
		r.Post("/logout", handler.LogoutHandler(db))
	})

	// CardDAV/WebDAV routes — all require authentication.
	r.Route("/dav", func(r chi.Router) {
		r.Use(middleware.RequireAuth(db))

		// Context path (RFC 6764 §6): PROPFIND returns current-user-principal.
		r.Options("/", handler.DAVOptions)
		r.MethodFunc("PROPFIND", "/", handler.DAVRootPropfind)

		// Principal resource (RFC 4918 + RFC 5397 + RFC 6352).
		r.Options("/principals/users/{username}/", handler.DAVOptions)
		r.MethodFunc("PROPFIND", "/principals/users/{username}/", handler.PrincipalPropfind(db))
	})

	srv := &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("starting contacthub", "addr", cfg.Server.Listen, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(shutCtx)
}

// bootstrapAdmin creates the initial admin user from config/env if no users exist.
func bootstrapAdmin(ctx context.Context, st *sqlite.DB, adminCfg *config.AdminConfig, logger *slog.Logger) error {
	if adminCfg == nil || adminCfg.Username == "" || adminCfg.Password == "" {
		return nil
	}

	n, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	if _, err := local.CreateUser(ctx, st, adminCfg.Username, "", adminCfg.Password, true); err != nil {
		return err
	}

	logger.Info("created admin user from config", "username", adminCfg.Username)
	return nil
}

func handleHealthz(db *sqlite.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"status": "ok"}

		if err := db.Ping(r.Context()); err != nil {
			resp["db"] = "error"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			resp["db"] = "ok"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		}

		_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck // write to ResponseWriter, error unrecoverable
	}
}

func newLogger(cfg config.LogConfig) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if cfg.Format == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(h)
}

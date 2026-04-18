package main

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/delaneyj/toolbelt/embeddednats"
	"github.com/gorilla/sessions"
	"github.com/nats-io/nats.go/jetstream"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Getenv, os.Stdout); err != nil {
		slog.Error("Error running server", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {

	slog.SetDefault(slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}

	sessionSecret := getenv("SESSION_SECRET")
	if sessionSecret == "" {
		return fmt.Errorf("SESSION_SECRET environment variable is required")
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("could not create static file system: %w", err)
	}

	decodedKey, err := base64.StdEncoding.DecodeString(sessionSecret)
	if err != nil {
		return fmt.Errorf("could not decode SESSION_SECRET: %w", err)
	}

	sessionStore := sessions.NewCookieStore(decodedKey)
	sessionStore.MaxAge(86400 * 30)
	sessionStore.Options.Path = "/"
	sessionStore.Options.HttpOnly = true
	sessionStore.Options.Secure = false
	sessionStore.Options.SameSite = http.SameSiteLaxMode

	ns, err := embeddednats.New(ctx, embeddednats.WithDirectory("/var/tmp/webserver"))
	if err != nil {
		return fmt.Errorf("could not create NATS server: %w", err)
	}
	ns.WaitForServer()

	nc, err := ns.Client()
	if err != nil {
		return fmt.Errorf("error creating nats client: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("error creating jetstream client: %w", err)
	}

	cfg := jetstream.KeyValueConfig{
		Bucket:      "rowingdata",
		Description: "Masters Rowing Data",
		Compression: true,
		TTL:         time.Hour,
		MaxBytes:    16 * 1024 * 1024,
	}

	s, err := newStore(ctx, js, cfg)
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	bus := newBusiness(s)

	app, err := newApplication(sessionStore, bus)
	if err != nil {
		return fmt.Errorf("could not create application: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "OK")
	})

	app.registerRoutes(mux)

	slog.Info("Server starting", "url", "http://localhost:"+port+"/masterscalc")
	if err := http.ListenAndServe(":"+port, mux); err != http.ErrServerClosed {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
}

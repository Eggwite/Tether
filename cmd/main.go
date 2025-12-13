package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tether/src/api"
	"tether/src/bot"
	"tether/src/logging"
	"tether/src/middleware"
	"tether/src/store"
	"tether/src/utils"
	ws "tether/src/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists (non-fatal if missing).
	_ = godotenv.Load()
	logging.Configure()

	port := getenv("PORT", "8080")
	st := store.NewPresenceStore()
	wsServer := ws.NewServer(st)

	r := chi.NewRouter()

	// Basic Middleware
	behindProxy := getenv("BEHIND_PROXY", "false") == "true"
	middleware.Setup(r, behindProxy)

	// Routes
	r.Get("/v1/users/{userID}", api.SnapshotHandler{Store: st}.ServeHTTP)
	r.Get("/healthz", api.HealthHandler{}.ServeHTTP)
	r.Handle("/socket", wsServer)
	// Custom 404 handler for API routes
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusNotFound, utils.PageNotFound())
	})
	// HTTP Server configuration
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	// Launch Discord bot
	discordSession, err := bot.Launch(os.Getenv("DISCORD_TOKEN"), st)
	if err != nil {
		logging.Log.WithError(err).Fatal("failed to start Discord bot")
	}

	go func() {
		logging.Log.WithField("addr", ":"+port).Info("server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Log.WithError(err).Fatal("http server error")
		}
	}()

	waitForShutdown(srv, discordSession, wsServer)
}

func waitForShutdown(srv *http.Server, discordSession interface{ Close() error }, wsServer interface{ Close() }) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logging.Log.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	if discordSession != nil {
		_ = discordSession.Close()
	}
	if wsServer != nil {
		wsServer.Close()
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

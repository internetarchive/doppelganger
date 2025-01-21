package server

import (
	"fmt"
	"net/http"

	"log/slog"

	"github.com/internetarchive/doppelganger/pkg/server/config"
	"github.com/internetarchive/doppelganger/pkg/server/handlers"
	"github.com/internetarchive/doppelganger/pkg/server/middlewares"
	"github.com/internetarchive/doppelganger/pkg/server/repositories"
)

func Start() {
	slog.Info("Starting HTTP server on port 8080")

	// Load config
	config, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config: %v", err)
		return
	}

	// Init ScyllaDB
	if err := repositories.Init(config); err != nil {
		slog.Error("Failed to initialize ScyllaDB: %v", err)
		return
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/records/", handlers.Records)

	http.Handle("/api/", middlewares.LogRequest(apiMux))

	// Metrics / healthcheck
	http.HandleFunc("/healthcheck", handlers.Healthcheck)

	http.ListenAndServe(fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port), nil)
}
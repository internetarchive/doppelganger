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
	// Load config
	config, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		return
	}

	// Init ScyllaDB
	if err := repositories.Init(config); err != nil {
		slog.Error("failed to initialize ScyllaDB", "err", err)
		return
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/records", handlers.Records)
	apiMux.HandleFunc("/api/records/", handlers.Records)

	http.Handle("/api/", middlewares.LogRequest(apiMux))

	// Metrics / healthcheck
	http.HandleFunc("/healthcheck", handlers.Healthcheck)

	slog.Info("starting HTTP server", "port", config.Server.Port)
	http.ListenAndServe(fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port), nil)
}

package server

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/hashicorp/consul/api"
	"github.com/internetarchive/doppelganger/pkg/server/config"
	"github.com/internetarchive/doppelganger/pkg/server/handlers"
	"github.com/internetarchive/doppelganger/pkg/server/middlewares"
	"github.com/internetarchive/doppelganger/pkg/server/repositories"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	// Register with Consul
	consulClient, serviceID, err := registerWithConsul(config)
	if err != nil {
		slog.Error("failed to register with Consul", "err", err)
		return
	}

	// Setup graceful shutdown to deregister from Consul
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		slog.Info("deregistering from Consul")
		consulClient.Agent().ServiceDeregister(serviceID)
		os.Exit(0)
	}()

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/records", handlers.Records)
	apiMux.HandleFunc("/api/records/", handlers.Records)

	http.Handle("/api/", middlewares.LogRequest(apiMux))

	// Metrics / healthcheck
	http.HandleFunc("/healthcheck", handlers.Healthcheck)
	http.Handle("/metrics", promhttp.Handler())

	slog.Info("starting HTTP server", "port", config.Server.Port)
	http.ListenAndServe(fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port), nil)
}

func registerWithConsul(config *config.Config) (*api.Client, string, error) {
	// Create Consul config to pass to new client
	consulConfig := api.DefaultConfig()

	// Set Consul address if specified in config
	if config.Consul.Address != "" {
		consulConfig.Address = config.Consul.Address
	}

	// Craete consul client
	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, "", err
	}

	// Generate service ID
	serviceID := fmt.Sprintf("doppelganger-%s-%d", config.Server.Host, config.Server.Port)

	// Define service registration
	registration := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    "doppelganger",
		Port:    config.Server.Port,
		Address: config.Server.Host,
		Tags:    []string{"api", "metrics"},
		Check: &api.AgentServiceCheck{
			HTTP:                           fmt.Sprintf("http://%s:%d/healthcheck", config.Server.Host, config.Server.Port),
			Interval:                       "30s",
			Timeout:                        "3s",
			DeregisterCriticalServiceAfter: "60s",
		},
	}

	// Register service
	err = client.Agent().ServiceRegister(registration)
	if err != nil {
		return nil, "", err
	}

	slog.Info("registered with Consul", "serviceID", serviceID)
	return client, serviceID, nil
}

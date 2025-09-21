package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server struct {
		Host  string
		Port  int
		Auth  bool
		Debug bool
	}
	Consul struct {
		Address string
	}
	Logging struct {
		Level            string
		PyroscopeAddress string
	}
	DB struct {
		ScyllaHosts             []string
		ScyllaKeyspace          string
		ScyllaReplicationClass  string
		ScyllaReplicationFactor int
	}
	CDX struct {
		URL              string
		Cookie           string
		ProcessInterval  int // in minutes
		BatchSize        int
	}
}

func LoadConfig() (*Config, error) {
	var config Config

	// Server configuration
	config.Server.Host = getEnv("SERVER_HOST", "0.0.0.0")
	config.Server.Port = getEnvAsInt("SERVER_PORT", 5000)
	config.Server.Debug = getEnvAsBool("SERVER_DEBUG", false)

	// Consul configuration
	config.Consul.Address = getEnv("CONSUL_ADDRESS", ":8500")

	// Logging configuration
	config.Logging.Level = getEnv("LOGGING_LEVEL", "info")
	config.Logging.PyroscopeAddress = getEnv("PYROSCOPE_ADDRESS", "")

	// Database configuration
	config.DB.ScyllaHosts = getEnvAsSlice("SCYLLA_HOSTS", []string{"localhost"})
	config.DB.ScyllaKeyspace = getEnv("SCYLLA_KEYSPACE", "crawlhq")
	config.DB.ScyllaReplicationClass = getEnv("SCYLLA_REPLICATION_CLASS", "SimpleStrategy")
	config.DB.ScyllaReplicationFactor = getEnvAsInt("SCYLLA_REPLICATION_FACTOR", 1)

	// CDX configuration
	config.CDX.URL = getEnv("CDX_URL", "")
	config.CDX.Cookie = getEnv("CDX_COOKIE", "")
	config.CDX.ProcessInterval = getEnvAsInt("CDX_PROCESS_INTERVAL", 10)
	config.CDX.BatchSize = getEnvAsInt("CDX_BATCH_SIZE", 100)

	return &config, nil
}

// Helper functions to read environment variables
func getEnv(key string, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(name string, defaultValue int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(name string, defaultValue bool) bool {
	valueStr := getEnv(name, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsSlice(name string, defaultValue []string) []string {
	valueStr := getEnv(name, "")
	if valueStr == "" {
		return defaultValue
	}
	return strings.Split(valueStr, ",")
}

package cmd

import (
	"log/slog"

	"github.com/internetarchive/doppelganger/pkg/server"
	"github.com/internetarchive/doppelganger/pkg/server/config"
	"github.com/internetarchive/doppelganger/pkg/server/repositories"
	"github.com/spf13/cobra"
)

var processPendingCmd = &cobra.Command{
	Use:   "process-pending",
	Short: "Process pending records and check CDX for availability",
	Long:  `This command processes records in the pending table and checks the CDX API to see if they are available. If found, it moves them to the records table with populated SHA1 and size fields.`,
	RunE:  runProcessPending,
}

func init() {
	rootCmd.AddCommand(processPendingCmd)
}

func runProcessPending(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load configuration", slog.String("error", err.Error()))
		return err
	}

	// Initialize repositories
	if err := repositories.Init(cfg); err != nil {
		slog.Error("failed to initialize repositories", slog.String("error", err.Error()))
		return err
	}

	slog.Info("Starting to process pending records...")

	// Create and run a one-time CDX processor
	cdxProcessor := server.NewCDXProcessor(cfg)
	cdxProcessor.ProcessBatch() // Run one batch synchronously

	slog.Info("Process pending completed")
	return nil
}

package cmd

import (
	"github.com/internetarchive/doppelganger/pkg/ingest"
	"github.com/spf13/cobra"
)

var concurrency int

// ingestCmd represents the ingest command
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest one or many CDX files",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ingest.Files(concurrency, args...)
	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)
	ingestCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 1, "Number of concurrent files to process")
}

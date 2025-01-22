package cmd

import (
	"github.com/internetarchive/doppelganger/pkg/ingest"
	"github.com/spf13/cobra"
)

var (
	concurrency int
	URL         string
)

// ingestCmd represents the ingest command
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest one or many CDX files",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ingest.Files(concurrency, URL, args...)
	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)
	ingestCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 1, "Number of concurrent files to process")
	ingestCmd.Flags().StringVarP(&URL, "url", "u", "http://localhost:8080", "Doppelganger server URL")
}

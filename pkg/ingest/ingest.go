package ingest

import (
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/internetarchive/gocdx"
)

func Files(concurrency int, files ...string) {
	for _, file := range files {
		file, err := os.Open(file)
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}
		defer file.Close()

		var skipped, valid int64

		parseStart := time.Now()
		records, err := gocdx.Parse(file, "CDX N b a m s k r M S V g")
		if err != nil {
			fmt.Println("Error parsing CDX file:", err)
			return
		}

		// Deduplicate records
		deduplicatedRecords, dedupedCount := deduplicateRecords(records)

		for _, record := range deduplicatedRecords {
			if record.StatusCode == 429 || record.StatusCode == 0 {
				atomic.AddInt64(&skipped, 1)
				continue
			}

			//slog.Info("CDX record", "url", record.OriginalURL, "timestamp", record.Timestamp, "status", record.StatusCode)
			atomic.AddInt64(&valid, 1)
		}

		slog.Info("CDX file parsed",
			"file", file.Name(),
			"duration", time.Since(parseStart),
			"valid", valid,
			"skipped", skipped,
			"deduped", dedupedCount,
			"total", len(records),
			"unique", len(deduplicatedRecords),
		)
	}
}

func deduplicateRecords(records []gocdx.Record) ([]gocdx.Record, int) {
	dedupMap := make(map[string]gocdx.Record)
	dedupedCount := 0

	for _, record := range records {
		checksum := record.NewStyleChecksum
		if existingRecord, ok := dedupMap[checksum]; ok {
			dedupedCount++
			if record.Timestamp.After(existingRecord.Timestamp) {
				dedupMap[checksum] = record
			}
		} else {
			dedupMap[checksum] = record
		}
	}

	deduplicatedRecords := make([]gocdx.Record, 0, len(dedupMap))
	for _, record := range dedupMap {
		deduplicatedRecords = append(deduplicatedRecords, record)
	}

	return deduplicatedRecords, dedupedCount
}

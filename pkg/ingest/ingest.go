package ingest

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/internetarchive/doppelganger/pkg/client"
	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/internetarchive/gocdx"
)

var (
	BATCH_SIZE          = 1000
	MINIMUM_RECORD_SIZE = int64(2000)
)

func Files(concurrency int, URL string, files ...string) {
	c := client.NewClient(URL)

	var batch []*models.Record
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
			// Skip records with status code 429 or 0
			// 0 will de-facto skip revisit records.
			if record.StatusCode == 429 ||
				record.StatusCode == 0 ||
				record.CompressedRecordSize < MINIMUM_RECORD_SIZE {
				atomic.AddInt64(&skipped, 1)
				continue
			}

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

		// Divide the records into batches of BATCH_SIZE
		for i := 0; i < len(deduplicatedRecords); i += BATCH_SIZE {
			batch = convertToModelRecords(deduplicatedRecords[i:min(i+BATCH_SIZE, len(deduplicatedRecords))])
			// Add the batch to the server
			if err := c.AddRecords(batch...); err != nil {
				fmt.Println("Error adding records:", err)
				return
			}
		}

		// Add the remaining records to the server
		if len(batch) > 0 {
			if err := c.AddRecords(batch...); err != nil {
				fmt.Println("Error adding records:", err)
				return
			}
		}
	}
}

func convertToModelRecords(records []gocdx.Record) []*models.Record {
	modelRecords := make([]*models.Record, len(records))
	for i, record := range records {
		date, err := strconv.Atoi(record.Timestamp.Format("20060102150405"))
		if err != nil {
			fmt.Println("Error converting date:", err)
			return nil
		}

		modelRecords[i] = &models.Record{
			ID:   record.NewStyleChecksum,
			URI:  record.OriginalURL,
			Date: int64(date),
		}
	}
	return modelRecords
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

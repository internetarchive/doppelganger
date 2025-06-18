package ingest

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/gzip"

	"github.com/internetarchive/doppelganger/pkg/client"
	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/internetarchive/gocdx"
)

var (
	BATCH_SIZE          = 1000
	MINIMUM_RECORD_SIZE = int64(2000)
	CHUNK_SIZE          = 100000
)

func Files(concurrency int, URL string, files ...string) {
	c := client.NewClient(URL)

	for _, file := range files {
		file, err := os.Open(file)
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}
		defer file.Close()

		var totalSkipped, totalValid int64
		var totalDedupedCount int
		var totalRecords int

		// Check if file is gzip compressed and decompress if needed
		var reader io.Reader = file
		if strings.HasSuffix(file.Name(), ".gz") {
			gzipReader, err := gzip.NewReader(file)
			if err != nil {
				fmt.Println("Error creating gzip reader:", err)
				return
			}
			defer gzipReader.Close()
			reader = gzipReader
		}

		parseStart := time.Now()

		// Process file in chunks
		if err := processFileInChunks(reader, c, &totalSkipped, &totalValid, &totalDedupedCount, &totalRecords); err != nil {
			fmt.Println("Error processing CDX file:", err)
			return
		}

		slog.Info("CDX file processed",
			"file", file.Name(),
			"duration", time.Since(parseStart),
			"valid", totalValid,
			"skipped", totalSkipped,
			"deduped", totalDedupedCount,
			"total", totalRecords,
		)
	}
}

func processFileInChunks(reader io.Reader, c *client.Client, totalSkipped, totalValid *int64, totalDedupedCount, totalRecords *int) error {
	scanner := bufio.NewScanner(reader)
	// Increase buffer size to handle large CDX lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size

	var lines []string
	chunkNum := 0

	for scanner.Scan() {
		lines = append(lines, scanner.Text())

		// Process chunk when we reach CHUNK_SIZE
		if len(lines) >= CHUNK_SIZE {
			if err := processChunk(lines, c, totalSkipped, totalValid, totalDedupedCount, totalRecords, chunkNum); err != nil {
				return err
			}
			lines = lines[:0] // Reset slice
			chunkNum++
		}
	}

	// Process remaining lines
	if len(lines) > 0 {
		if err := processChunk(lines, c, totalSkipped, totalValid, totalDedupedCount, totalRecords, chunkNum); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func processChunk(lines []string, c *client.Client, totalSkipped, totalValid *int64, totalDedupedCount, totalRecords *int, chunkNum int) error {
	// Convert lines back to reader for gocdx.Parse
	chunkData := strings.Join(lines, "\n")
	chunkReader := strings.NewReader(chunkData)

	var skipped, valid int64

	// Parse the chunk
	records, err := gocdx.Parse(chunkReader, "CDX N b a m s k r M S V g")
	if err != nil {
		return fmt.Errorf("error parsing CDX chunk %d: %w", chunkNum, err)
	}

	*totalRecords += len(records)

	// Deduplicate records in this chunk
	deduplicatedRecords, dedupedCount := deduplicateRecords(records)
	*totalDedupedCount += dedupedCount

	var validRecords []gocdx.Record

	for _, record := range deduplicatedRecords {
		// Skip records with status code 429 or 0
		// 0 will de-facto skip revisit records.
		// We also set a minimum record size to avoid performance issues on certain revisit records.
		if record.StatusCode == 429 ||
			record.StatusCode == 0 ||
			record.CompressedRecordSize < MINIMUM_RECORD_SIZE {
			atomic.AddInt64(&skipped, 1)
			continue
		} else {
			// Filter valid records for batch processing
			validRecords = append(validRecords, record)
			atomic.AddInt64(&valid, 1)
		}
	}

	atomic.AddInt64(totalSkipped, skipped)
	atomic.AddInt64(totalValid, valid)

	slog.Info("Processed chunk",
		"chunk", chunkNum,
		"records", len(records),
		"unique", len(deduplicatedRecords),
		"valid", valid,
		"skipped", skipped,
		"deduped", dedupedCount,
	)

	// Divide the valid records into batches of BATCH_SIZE
	for i := 0; i < len(validRecords); i += BATCH_SIZE {
		batch := convertToModelRecords(validRecords[i:min(i+BATCH_SIZE, len(validRecords))])
		if batch == nil {
			return fmt.Errorf("error converting records to model")
		}

		// Add the batch to the server
		if err := c.AddRecords(batch...); err != nil {
			return fmt.Errorf("error adding records: %w", err)
		}
	}

	return nil
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

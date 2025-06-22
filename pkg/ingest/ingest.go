package ingest

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/gzip"

	"github.com/internetarchive/doppelganger/pkg/client"
	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/internetarchive/gocdx"
)

var (
	// BATCH_SIZE can be an issue on larger URLs? Work to dynamically adjust in the future?
	// TODO: dynamically adjust
	BATCH_SIZE          = 850
	MINIMUM_RECORD_SIZE = int64(2000)
	CHUNK_SIZE          = 100000
)

type ChunkData struct {
	Lines   []string
	ChunkID int
}

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
		var totalDedupedCount int64
		var totalRecords int64

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

		// Process file in chunks with concurrency
		if err := processFileInChunksConcurrent(reader, c, &totalSkipped, &totalValid, &totalDedupedCount, &totalRecords, concurrency); err != nil {
			fmt.Println("Error processing CDX file:", err)
			return
		}

		slog.Info("CDX file processed",
			"file", file.Name(),
			"duration", time.Since(parseStart),
			"total", totalRecords,
			"deduped", totalDedupedCount,
			"skipped", totalSkipped,
			"valid", totalValid,
		)
	}
}

func processFileInChunksConcurrent(reader io.Reader, c *client.Client, totalSkipped, totalValid, totalDedupedCount, totalRecords *int64, concurrency int) error {
	scanner := bufio.NewScanner(reader)
	// Increase buffer size to handle large CDX lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size

	// Create channels for chunk processing
	chunkChan := make(chan ChunkData, concurrency*2) // Buffer a few chunks
	errChan := make(chan error, concurrency)

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chunkWorker(chunkChan, c, totalSkipped, totalValid, totalDedupedCount, totalRecords, errChan)
		}()
	}

	// Read file and send chunks to workers
	go func() {
		defer close(chunkChan)

		var lines []string
		chunkNum := 0

		for scanner.Scan() {
			lines = append(lines, scanner.Text())

			// Send chunk when we reach CHUNK_SIZE
			if len(lines) >= CHUNK_SIZE {
				// Make a copy of the lines slice to avoid race conditions
				chunkLines := make([]string, len(lines))
				copy(chunkLines, lines)

				select {
				case chunkChan <- ChunkData{Lines: chunkLines, ChunkID: chunkNum}:
					lines = lines[:0] // Reset slice
					chunkNum++
				case err := <-errChan:
					slog.Error("Error from worker", "error", err)
					return
				}
			}
		}

		// Send remaining lines if any
		if len(lines) > 0 {
			chunkLines := make([]string, len(lines))
			copy(chunkLines, lines)

			select {
			case chunkChan <- ChunkData{Lines: chunkLines, ChunkID: chunkNum}:
			case err := <-errChan:
				slog.Error("Error from worker", "error", err)
				return
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case errChan <- fmt.Errorf("scanner error: %w", err):
			default:
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func chunkWorker(chunkChan <-chan ChunkData, c *client.Client, totalSkipped, totalValid, totalDedupedCount, totalRecords *int64, errChan chan<- error) {
	for chunk := range chunkChan {
		if err := processChunk(chunk.Lines, c, totalSkipped, totalValid, totalDedupedCount, totalRecords, chunk.ChunkID); err != nil {
			select {
			case errChan <- err:
			default: // Don't block if error channel is full
			}
			return
		}
	}
}

func processChunk(lines []string, c *client.Client, totalSkipped, totalValid, totalDedupedCount, totalRecords *int64, chunkNum int) error {
	// Convert lines back to reader for gocdx.Parse
	chunkData := strings.Join(lines, "\n")
	chunkReader := strings.NewReader(chunkData)

	var skipped, valid int64

	// Parse the chunk
	records, err := gocdx.Parse(chunkReader, "CDX N b a m s k r M S V g")
	if err != nil {
		return fmt.Errorf("error parsing CDX chunk %d: %w", chunkNum, err)
	}

	// Clear chunkData immediately after parsing to free memory
	chunkData = ""

	atomic.AddInt64(totalRecords, int64(len(records)))

	// Count records processed in this chunk
	chunkRecordCount := int64(len(records))

	// Deduplicate records in this chunk
	deduplicatedRecords, dedupedCount := deduplicateRecords(records)
	atomic.AddInt64(totalDedupedCount, int64(dedupedCount))

	// Free memory from original records slice (it is no longer used in this function after deduplication.)
	records = nil

	var validRecords []gocdx.Record

	for _, record := range deduplicatedRecords {
		// Skip records with status code 429 or 0
		// 0 will de-facto skip revisit records.
		// We also set a minimum record size to avoid performance issues on certain revisit records.
		if record.StatusCode == 429 ||
			record.StatusCode == 0 ||
			record.CompressedRecordSize < MINIMUM_RECORD_SIZE ||
			record.NewStyleChecksum == "3I42H3S6NNFQ2MSVX7XZKYAYSCX5QBYJ" {
			skipped++
			continue
		} else {
			// Filter valid records for batch processing
			validRecords = append(validRecords, record)
			valid++
		}
	}

	atomic.AddInt64(totalSkipped, skipped)
	atomic.AddInt64(totalValid, valid)

	// Free memory from deduplicatedRecords slice
	deduplicatedRecords = nil

	// Divide the valid records into batches of BATCH_SIZE
	for i := 0; i < len(validRecords); i += BATCH_SIZE {
		end := min(i+BATCH_SIZE, len(validRecords))
		batch := convertToModelRecords(validRecords[i:end])
		if batch == nil {
			return fmt.Errorf("error converting records to model")
		}

		// Add the batch to the server
		for {
			if err := c.AddRecords(batch...); err != nil {
				slog.Warn("Error adding records, retrying", "error", err)
				time.Sleep(time.Second) // Brief delay before retry
				continue
			}
			break // Success, exit retry loop
		}

		// Clear batch reference after successful submission
		batch = nil

		// Clear the processed slice segment to free memory incrementally
		for j := i; j < end; j++ {
			validRecords[j] = gocdx.Record{} // Zero out the record
		}
	}

	slog.Info("Processed chunk",
		"chunk", chunkNum,
		"records", chunkRecordCount,
		"deduped", dedupedCount,
		"skipped", skipped,
		"valid", valid,
		"totalRecords", atomic.LoadInt64(totalRecords),
	)

	// Explicitly clear validRecords when batch has been submitted.
	validRecords = nil

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

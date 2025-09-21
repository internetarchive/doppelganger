package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/internetarchive/doppelganger/pkg/server/config"
	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/internetarchive/doppelganger/pkg/server/repositories"
)

// CDXProcessor handles the recurring task of processing pending records
type CDXProcessor struct {
	config    *config.Config
	ticker    *time.Ticker
	ctx       context.Context
	cancel    context.CancelFunc
	isRunning bool
}

// NewCDXProcessor creates a new CDX processor instance
func NewCDXProcessor(cfg *config.Config) *CDXProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &CDXProcessor{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the recurring CDX processing task
func (p *CDXProcessor) Start() {
	if p.isRunning {
		slog.Warn("CDX processor is already running")
		return
	}

	if p.config.CDX.URL == "" {
		slog.Warn("CDX URL not configured, skipping CDX processing")
		return
	}

	p.isRunning = true
	interval := time.Duration(p.config.CDX.ProcessInterval) * time.Minute
	p.ticker = time.NewTicker(interval)

	slog.Info("Starting CDX processor",
		slog.String("interval", interval.String()),
		slog.Int("batch_size", p.config.CDX.BatchSize))

	go p.run()
}

// Stop halts the CDX processing task
func (p *CDXProcessor) Stop() {
	if !p.isRunning {
		return
	}

	slog.Info("Stopping CDX processor")
	p.cancel()
	if p.ticker != nil {
		p.ticker.Stop()
	}
	p.isRunning = false
}

// run executes the main processing loop
func (p *CDXProcessor) run() {
	defer func() {
		p.isRunning = false
	}()

	// Process immediately on start
	p.processBatchInternal()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.ticker.C:
			p.processBatchInternal()
		}
	}
}

// processBatch processes a batch of pending records
func (p *CDXProcessor) ProcessBatch() {
	slog.Info("Starting CDX batch processing")

	pendingRecords, err := repositories.GetAllPendingRecords()
	if err != nil {
		slog.Error("failed to retrieve pending records", slog.String("error", err.Error()))
		return
	}

	if len(pendingRecords) == 0 {
		slog.Info("No pending records to process")
		return
	}

	slog.Info("Processing pending records", slog.Int("count", len(pendingRecords)))

	processed := 0
	batchSize := p.config.CDX.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	for i, pending := range pendingRecords {
		// Process in batches to avoid overwhelming the system
		if processed >= batchSize {
			slog.Info("Reached batch size limit", slog.Int("processed", processed))
			break
		}

		if err := p.processRecord(pending); err != nil {
			slog.Error("failed to process pending record",
				slog.String("id", pending.ID),
				slog.String("uri", pending.URI),
				slog.String("error", err.Error()))
		} else {
			processed++
		}

		// Add a small delay between requests to be respectful to the CDX API
		if i < len(pendingRecords)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	slog.Info("CDX batch processing completed", slog.Int("processed", processed))
}

// processBatchInternal is the internal method used by the recurring task
func (p *CDXProcessor) processBatchInternal() {
	p.ProcessBatch()
}

// processRecord processes a single pending record
func (p *CDXProcessor) processRecord(pending *models.Record) error {
	// Use SHA1 from pending record as digest for CDX lookup
	if pending.SHA1 == "" {
		slog.Info("Skipping record without SHA1", slog.String("id", pending.ID))
		return nil
	}

	// Convert date to string format
	dateStr := fmt.Sprintf("%d", pending.Date)

	cdxStatus, err := checkCDX(p.config.CDX.URL, pending.SHA1, pending.URI, dateStr, p.config.CDX.Cookie)
	if err != nil {
		return err
	}

	// If we found a matching record in CDX, move it to records table
	if cdxStatus {
		record := &models.Record{
			ID:   pending.ID,
			URI:  pending.URI,
			Date: pending.Date,
			SHA1: pending.SHA1,
			Size: pending.Size,
		}

		if err := repositories.MovePendingToRecords(pending.ID, record); err != nil {
			return err
		}

		slog.Info("Successfully moved record from pending to records",
			slog.String("id", pending.ID),
			slog.String("uri", pending.URI),
			slog.Int64("size", pending.Size))
	} else {
		slog.Debug("No CDX match found for record",
			slog.String("id", pending.ID),
			slog.String("uri", pending.URI))
	}

	return nil
}

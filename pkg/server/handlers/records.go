package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/internetarchive/doppelganger/pkg/server/repositories"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	doppelganger_successful_hits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "doppelganger_successful_hits",
		Help: "The total number of successful responses from Doppelganger deduplication",
	})
	doppelganger_non_matching_hits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "doppelganger_non_matching_hits",
		Help: "The total number of successful responses but with different Target-URIs from Doppelganger deduplication",
	})
	doppelganger_missing_hits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "doppelganger_missing_hits",
		Help: "The total number of unsuccessful responses from Doppelganger deduplication",
	})
)

func Records(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:

		// Check for URI query parameter
		uri := r.URL.Query().Get("uri")
		ID := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/api/records"), "/")

		if ID == "" && uri == "" {
			http.Error(w, "invalid ID or URI", http.StatusBadRequest)
			return
		}

		// First, try to get the record from the records table
		record, err := repositories.GetRecord(ID)
		if err != nil {
			if err == repositories.ErrRecordNotFound {
				http.Error(w, "record not found", http.StatusNotFound)
				doppelganger_missing_hits.Inc()
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		doppelganger_successful_hits.Inc()

		if uri != record.URI {
			doppelganger_non_matching_hits.Inc()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(record)
	case http.MethodPost:

		// Parse JSON body
		var requestData struct {
			ID   string `json:"id"`
			URI  string `json:"uri"`
			SHA1 string `json:"sha1"`
			Size int64  `json:"size"`
			Date int64  `json:"date"`
		}

		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if requestData.ID == "" || requestData.URI == "" || requestData.SHA1 == "" || requestData.Date == 0 || requestData.Size == 0 {
			http.Error(w, "id, uri, sha1, date, and size are required", http.StatusBadRequest)
			return
		}

		// Validate field formats and values
		// Validate SHA1 format (40 hexadecimal characters)
		sha1Regex := regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
		if !sha1Regex.MatchString(requestData.SHA1) {
			http.Error(w, "sha1 must be a valid 40-character hexadecimal hash", http.StatusBadRequest)
			return
		}

		// Validate URI format
		if _, err := url.ParseRequestURI(requestData.URI); err != nil {
			http.Error(w, "uri must be a valid URI", http.StatusBadRequest)
			return
		}

		// Validate Date is positive (assuming Unix timestamp)
		if requestData.Date <= 0 {
			http.Error(w, "date must be a positive timestamp", http.StatusBadRequest)
			return
		}

		// Validate Size is positive
		if requestData.Size <= 0 {
			http.Error(w, "size must be a positive number", http.StatusBadRequest)
			return
		}

		// Validate ID format (basic check for reasonable characters)
		if strings.TrimSpace(requestData.ID) != requestData.ID || len(requestData.ID) > 255 {
			http.Error(w, "id must not contain leading/trailing whitespace and must be under 256 characters", http.StatusBadRequest)
			return
		}

		// Check if record already exists in records table
		existingRecord, err := repositories.GetRecord(requestData.ID)
		if err == nil {
			// Record exists, return it
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(existingRecord)
			return
		}

		if err != repositories.ErrRecordNotFound {
			// Unexpected error
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Record not found in records table, add it to pending table
		pendingRecord := &models.Record{
			ID:   requestData.ID,
			URI:  requestData.URI,
			Date: requestData.Date,
			SHA1: requestData.SHA1,
			Size: requestData.Size,
		}

		// Try to add to pending table
		if err := repositories.AddPendingRecord(pendingRecord); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Return 404 to indicate record was not found but has been queued for processing
		http.Error(w, "record not found, added to pending queue", http.StatusNotFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func BulkRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var records []models.Record

	if err := json.NewDecoder(r.Body).Decode(&records); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var recordPtrs []*models.Record
	for i := range records {
		recordPtrs = append(recordPtrs, &records[i])
	}
	err := repositories.AddRecords(recordPtrs...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

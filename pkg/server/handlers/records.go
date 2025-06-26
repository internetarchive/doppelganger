package handlers

import (
	"encoding/json"
	"net/http"
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
	ID := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/api/records"), "/")
	if r.Method == http.MethodPost && ID != "" {
		http.Error(w, "invalid path for POST method", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Check for URI query parameter
		uri := r.URL.Query().Get("uri")

		if ID == "" && uri == "" {
			http.Error(w, "invalid ID or URI", http.StatusBadRequest)
			return
		}

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
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

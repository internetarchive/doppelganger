package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/internetarchive/doppelganger/pkg/server/repositories"
)

func Records(w http.ResponseWriter, r *http.Request) {
	ID := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/api/records"), "/")
	if r.Method == http.MethodPost && ID != "" {
		http.Error(w, "invalid path for POST method", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if ID == "" {
			http.Error(w, "invalid ID", http.StatusBadRequest)
			return
		}

		record, err := repositories.GetRecord(ID)
		if err != nil {
			if err == repositories.ErrRecordNotFound {
				http.Error(w, "record not found", http.StatusNotFound)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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

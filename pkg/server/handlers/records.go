package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/internetarchive/doppelganger/pkg/server/models"
	"github.com/internetarchive/doppelganger/pkg/server/repositories"
)

func Records(w http.ResponseWriter, r *http.Request) {
	// Extract the record ID from the URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	ID := pathParts[len(pathParts)-1]

	if ID == "" {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
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
		var record models.Record

		if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		record.ID = ID

		err := repositories.AddRecord(&record)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(record)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

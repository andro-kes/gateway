package handlers

import (
	"net/http"
)

func CheckHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write([]byte(`{"status": "healthy"}`))
	if err != nil {
		http.Error(w, "Failed to connect to auth service", http.StatusServiceUnavailable)
	}
}

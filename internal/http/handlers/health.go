package handlers

import (
	"net/http"
)

func CheckHealth(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("go"))
	if err != nil {
		http.Error(w, "Failed to connect to auth service", http.StatusServiceUnavailable)
	}
}

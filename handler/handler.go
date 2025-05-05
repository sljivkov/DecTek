package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

func pricesHandler(w http.ResponseWriter, r *http.Request) {
	// Wait until prices are ready
	select {
	case <-readyCh:
	case <-time.After(3 * time.Second): // fallback timeout
		http.Error(w, "prices not ready", http.StatusServiceUnavailable)
		return
	}

	mu.RLock()
	defer mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiPrices)
}

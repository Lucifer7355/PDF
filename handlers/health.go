package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "PDF Toolbox is live",
		"time":    time.Now().Format(time.RFC3339),
	})
}

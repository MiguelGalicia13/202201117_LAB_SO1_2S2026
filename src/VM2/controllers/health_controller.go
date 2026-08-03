package controllers

import (
	"encoding/json"
	"net/http"
	"time"
	"api3/models"
)

// HealthCheck responde con el estado de salud de la API
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := models.HealthResponse{
		Status:    "UP",
		Service:   "API3 is ready",
		Timestamp: time.Now().Format(time.RFC3339),
		Vm: 3,
		Carnet: 202201117,
	}

	json.NewEncoder(w).Encode(response)
}

package main

import (
	"bytes"
	"encoding/json"
	"github.com/MiguelGalicia13/202201117_LAB_SO1_2S2026/Proyecto3/internals/report"
	"io"
	"log"
	"net/http"
	"time"
)

var client = &http.Client{
	Timeout: 4 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	},
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /report", recieveReport)
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
func recieveReport(w http.ResponseWriter, r *http.Request) {
	value, err := report.Decode(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "Could not code report", http.StatusInternalServerError)
		return
	}
	req, err := http.NewRequestWithContext(
		r.Context(), http.MethodPost,
		"http://127.0.0.1:8081/send", bytes.NewReader(body),
	)
	if err != nil {
		http.Error(w, "Could not create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "gRPC unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, "gRPC unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("could not write response: %v", err)
	}

}

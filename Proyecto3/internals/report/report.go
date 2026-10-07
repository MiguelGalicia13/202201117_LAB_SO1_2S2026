package report

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Report struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

type reportPayload struct {
	Country         string `json:"country"`
	WarplanesInAir  *int32 `json:"warplanes_in_air"`
	WarshipsInWater *int32 `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

func (r Report) Validate() error {
	switch r.Country {
	case "USA", "RUS", "CHN", "ESP", "GTM":
	default:
		return fmt.Errorf("invalid country: %s", r.Country)
	}
	if r.WarplanesInAir < 0 {
		return fmt.Errorf("warplanes_in_air must be non-negative")
	}
	if r.WarshipsInWater < 0 {
		return fmt.Errorf("warships_in_water must be non-negative")
	}
	if !strings.HasSuffix(r.Timestamp, "Z") && !strings.HasSuffix(r.Timestamp, "+00:00") {
		return fmt.Errorf("invalid timestamp: %s", r.Timestamp)
	}
	if _, err := time.Parse(time.RFC3339, r.Timestamp); err != nil {
		return fmt.Errorf("invalid timestamp: %s", r.Timestamp)
	}
	return nil
}

func Decode(w http.ResponseWriter, r *http.Request) (Report, error) {
	var payload reportPayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return Report{}, err
	}

	var extra any
	err := decoder.Decode(&extra)
	if err == nil {
		return Report{}, fmt.Errorf("only one JSON document is allowed")
	}
	if err != io.EOF {
		return Report{}, err
	}

	if payload.WarplanesInAir == nil || payload.WarshipsInWater == nil {
		return Report{}, fmt.Errorf("warplanes_in_air and warships_in_water are required")
	}

	value := Report{
		Country:         payload.Country,
		WarplanesInAir:  *payload.WarplanesInAir,
		WarshipsInWater: *payload.WarshipsInWater,
		Timestamp:       payload.Timestamp,
	}
	if err := value.Validate(); err != nil {
		return Report{}, err
	}
	return value, nil
}

package models

// HealthResponse representa la respuesta del endpoint de salud
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Vm int `json:"VM"`
	Carnet int `json:"carnet"`
}

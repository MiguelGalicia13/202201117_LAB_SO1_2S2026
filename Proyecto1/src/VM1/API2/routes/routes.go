package routes

import (
	"net/http"
	"API2/controllers"
)

// SetupRouter configura todas las rutas de la API
func SetupRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", controllers.HealthCheck)
	mux.HandleFunc("GET /api2/202201117/call-api1", controllers.CallApi1)
	mux.HandleFunc("GET /api2/202201117/call-api3", controllers.CallApi3)
	return mux
}

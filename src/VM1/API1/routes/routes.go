package routes

import (
	"net/http"
	"api1/controllers"
)

// SetupRouter configura todas las rutas de la API
func SetupRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", controllers.HealthCheck)
	mux.HandleFunc("GET /api1/202201117/call-api2", controllers.CallApi2)
	mux.HandleFunc("GET /api1/202201117/call-api3", controllers.CallApi3)
	return mux
}

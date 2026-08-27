package main
import (
	"log"
	"net/http"
	"api3/routes"
)

func main() {
	router := routes.SetupRouter()

	port := ":8080"
	log.Printf("API3 corriendo en http://localhost%s", port)

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
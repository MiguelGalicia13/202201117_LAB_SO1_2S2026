package main
import (
	"log"
	"net/http"
	"API2/routes"
)

func main() {
	router := routes.SetupRouter()

	port := ":8081"
	log.Printf("API2 corriendo en http://localhost%s", port)

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
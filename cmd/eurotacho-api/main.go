package main

import (
	"log"
	"net/http"
	"os"

	// Carga las claves ERCA al inicializar
	_ "github.com/traconiq/tachoparser/internal/pkg/certificates"

	"github.com/traconiq/tachoparser/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", api.HandleHealth)
	mux.HandleFunc("/api/v1/parse", api.HandleParse)

	log.Printf("EuroTacho API v%s escuchando en :%s", api.ParserVersion, port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Error iniciando servidor: %v", err)
	}
}

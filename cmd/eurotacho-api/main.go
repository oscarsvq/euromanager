package main

import (
	"log"
	"net/http"
	"os"

	// Carga las claves ERCA al inicializar
	_ "github.com/traconiq/tachoparser/internal/pkg/certificates"

	"github.com/traconiq/tachoparser/internal/api"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", api.HandleHealth)
	mux.HandleFunc("/api/v1/parse", api.HandleParse)

	// Wrapper CORS para desarrollo local
	handler := corsMiddleware(mux)

	log.Printf("EuroTacho API v%s escuchando en :%s", api.ParserVersion, port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Error iniciando servidor: %v", err)
	}
}

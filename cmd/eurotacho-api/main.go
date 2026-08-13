package main

import (
	"log"
	"net/http"
	"os"
	"time"

	// Carga las claves ERCA al inicializar
	_ "github.com/traconiq/tachoparser/internal/pkg/certificates"

	"github.com/traconiq/tachoparser/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	apiKey := os.Getenv("PARSER_API_KEY")
	if apiKey == "" && os.Getenv("PARSER_ALLOW_INSECURE") != "1" {
		// Fallo cerrado a proposito: este servicio queda expuesto en internet y
		// procesa datos personales de conductores. Si la clave falta, el arranque
		// se detiene en vez de levantar un parser abierto a cualquiera. Para
		// desarrollo local, PARSER_ALLOW_INSECURE=1 lo permite explicitamente.
		log.Fatal("PARSER_API_KEY no configurada. Define la clave, o PARSER_ALLOW_INSECURE=1 para desarrollo local sin autenticacion.")
	}
	if apiKey == "" {
		log.Print("AVISO: arrancando SIN autenticacion (PARSER_ALLOW_INSECURE=1). No usar en produccion.")
	}

	origins := api.ParseOrigins(os.Getenv("PARSER_ALLOWED_ORIGINS"))
	if len(origins) > 0 {
		log.Printf("CORS habilitado para: %v", origins)
	}

	mux := http.NewServeMux()
	// /health queda fuera de la autenticacion: lo consultan los health checks de
	// la plataforma, que no llevan la clave. Solo expone estado y version.
	mux.HandleFunc("/api/v1/health", api.HandleHealth)
	mux.Handle("/api/v1/parse", api.RequireAPIKey(apiKey, http.HandlerFunc(api.HandleParse)))

	// CORS por fuera de la clave: el preflight del navegador nunca lleva
	// cabeceras personalizadas, asi que un OPTIONS seria rechazado con 401.
	handler := api.CORS(origins, mux)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
		// Sin timeouts, una conexion lenta puede retener un worker
		// indefinidamente (Slowloris). ReadTimeout cubre la subida de hasta
		// 10 MB; WriteTimeout deja margen para serializar el JSON de un VU con
		// velocidad detallada, que son decenas de miles de puntos.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      180 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16, // 64 KB
	}

	// Sin "v" delante: la version ya la trae si corresponde (v1.2.3, dev+abc123).
	log.Printf("EuroTacho API %s escuchando en :%s", api.ParserVersion, port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Error iniciando servidor: %v", err)
	}
}

package api

import (
	"encoding/json"
	"net/http"
)

// ParserVersion indica la versión actual del parser
const ParserVersion = "0.1.0"

// HandleHealth retorna el estado del servicio y la versión del parser
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":         "ok",
		"parser_version": ParserVersion,
	})
}

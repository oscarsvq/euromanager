package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// APIKeyHeader es la cabecera en la que se espera la clave del servicio.
const APIKeyHeader = "X-Api-Key"

// RequireAPIKey exige que la peticion presente la clave del servicio.
//
// Con `key` vacia devuelve el handler sin envolver: es el modo de desarrollo
// local. La decision de si ese modo esta permitido NO se toma aqui sino en el
// arranque del servidor (cmd/eurotacho-api), que se niega a arrancar sin clave
// salvo opt-in explicito. Asi el fallo por defecto es cerrado: olvidar la
// variable en produccion impide arrancar, no deja el parser abierto.
//
// La comparacion es en tiempo constante: el parser queda expuesto en internet y
// una comparacion normal filtra la clave byte a byte por diferencia de tiempos.
func RequireAPIKey(key string, next http.Handler) http.Handler {
	if key == "" {
		return next
	}
	expected := []byte(key)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get(APIKeyHeader))
		// ConstantTimeCompare ya devuelve 0 si las longitudes difieren, asi que
		// tambien cubre el caso de una clave que sea prefijo de la correcta.
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Clave de API ausente o incorrecta")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ParseOrigins convierte la lista CSV de la variable de entorno en un slice,
// descartando espacios y entradas vacias.
func ParseOrigins(csv string) []string {
	var out []string
	for _, parte := range strings.Split(csv, ",") {
		if s := strings.TrimSpace(parte); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CORS emite cabeceras solo para los origenes de la lista blanca.
//
// Con la lista vacia no emite ninguna cabecera CORS, que es el estado correcto
// en produccion: alli el unico cliente es la Edge Function, que llama de
// servidor a servidor y no pasa por la politica del navegador. El comodin "*"
// que habia antes convertia el parser en invocable desde cualquier pagina web.
//
// Debe montarse POR FUERA de RequireAPIKey: el navegador no envia cabeceras
// personalizadas en el preflight, asi que un OPTIONS nunca lleva X-Api-Key y
// seria rechazado con 401 antes de llegar aqui.
func CORS(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origenPermitido(allowed, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+APIKeyHeader)
			w.Header().Set("Access-Control-Max-Age", "600")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func origenPermitido(allowed []string, origin string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}

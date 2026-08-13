package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// okHandler es un handler trivial que marca que la peticion llego al final de
// la cadena de middlewares.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("llego"))
	})
}

func TestRequireAPIKeyRechazaSinCabecera(t *testing.T) {
	h := RequireAPIKey("clave-secreta", okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperaba 401 sin cabecera, obtuve %d", rec.Code)
	}
	if rec.Body.String() == "llego" {
		t.Fatal("la peticion no debio alcanzar el handler")
	}
}

func TestRequireAPIKeyRechazaClaveIncorrecta(t *testing.T) {
	h := RequireAPIKey("clave-secreta", okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", nil)
	req.Header.Set("X-Api-Key", "clave-equivocada")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperaba 401 con clave incorrecta, obtuve %d", rec.Code)
	}
}

// Una clave que es prefijo de la correcta no debe pasar: cubre el fallo clasico
// de comparar con strings.HasPrefix o de cortar la comparacion antes de tiempo.
func TestRequireAPIKeyRechazaPrefijo(t *testing.T) {
	h := RequireAPIKey("clave-secreta", okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", nil)
	req.Header.Set("X-Api-Key", "clave")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperaba 401 con clave truncada, obtuve %d", rec.Code)
	}
}

func TestRequireAPIKeyAceptaClaveCorrecta(t *testing.T) {
	h := RequireAPIKey("clave-secreta", okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", nil)
	req.Header.Set("X-Api-Key", "clave-secreta")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200 con la clave correcta, obtuve %d", rec.Code)
	}
}

// Con clave vacia el middleware NO debe envolver nada: es el modo dev explicito,
// y quien decide si se permite es el arranque del servidor, no el middleware.
func TestRequireAPIKeyVaciaDejaPasar(t *testing.T) {
	h := RequireAPIKey("", okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200 en modo sin clave, obtuve %d", rec.Code)
	}
}

func TestCORSSinOrigenesNoEmiteCabeceras(t *testing.T) {
	h := CORS(nil, okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", nil)
	req.Header.Set("Origin", "https://atacante.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("no debia emitir cabecera CORS, obtuve %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("la peticion debia pasar igualmente, obtuve %d", rec.Code)
	}
}

func TestCORSReflejaSoloOrigenPermitido(t *testing.T) {
	h := CORS([]string{"http://localhost:5173"}, okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("esperaba reflejar el origen permitido, obtuve %q", got)
	}
	// Nunca comodin: con credenciales o no, "*" es lo que la auditoria marco.
	if rec.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Fatal("no debe emitirse el comodin")
	}
}

func TestCORSIgnoraOrigenNoPermitido(t *testing.T) {
	h := CORS([]string{"http://localhost:5173"}, okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", nil)
	req.Header.Set("Origin", "https://atacante.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("no debia autorizar un origen desconocido, obtuve %q", got)
	}
}

// El preflight debe anunciar X-Api-Key: si no, el navegador bloquea la peticion
// real en cuanto el parser exige la clave.
func TestCORSPreflightAnunciaApiKey(t *testing.T) {
	h := CORS([]string{"http://localhost:5173"}, okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/parse", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperaba 204 en preflight, obtuve %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-Api-Key") {
		t.Fatalf("el preflight debe permitir X-Api-Key, obtuve %q", got)
	}
}

func TestParseOrigins(t *testing.T) {
	casos := []struct {
		entrada string
		quiero  int
	}{
		{"", 0},
		{"   ", 0},
		{"http://localhost:5173", 1},
		{"http://localhost:5173,https://app.eurotacho.com", 2},
		{" http://localhost:5173 , https://app.eurotacho.com ", 2},
	}
	for _, c := range casos {
		got := ParseOrigins(c.entrada)
		if len(got) != c.quiero {
			t.Errorf("ParseOrigins(%q) = %v (%d), esperaba %d", c.entrada, got, len(got), c.quiero)
		}
	}
}

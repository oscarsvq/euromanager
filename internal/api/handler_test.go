package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	// Importar certificados para que la verificación de firmas funcione
	_ "github.com/traconiq/tachoparser/internal/pkg/certificates"
)

// testdataDir es la ruta relativa a los archivos de prueba desde internal/api/
const testdataDir = "../../testdata/"

func TestParseDriverCard(t *testing.T) {
	path := testdataDir + "C_E07118343G000000_E_20260412_2105.TGD"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("archivo de tarjeta no encontrado en %s: %v", path, err)
	}

	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(data))
	rec := httptest.NewRecorder()

	HandleParse(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba HTTP 200, obtuve %d: %s", rec.Code, rec.Body.String())
	}

	var resp ParseResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("error decodificando respuesta JSON: %v", err)
	}

	if resp.FileType != "card" {
		t.Errorf("esperaba file_type \"card\", obtuve %q", resp.FileType)
	}

	if resp.SignatureVerification.Status != "valid" {
		t.Errorf("esperaba signature_verification.status \"valid\", obtuve %q", resp.SignatureVerification.Status)
	}

	if resp.ParserVersion != ParserVersion {
		t.Errorf("esperaba parser_version %q, obtuve %q", ParserVersion, resp.ParserVersion)
	}
}

func TestParseVehicleUnit(t *testing.T) {
	path := testdataDir + "V_7118JST_E_20260416_1139.TGD"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("archivo VU no encontrado en %s: %v", path, err)
	}

	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(data))
	rec := httptest.NewRecorder()

	HandleParse(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba HTTP 200, obtuve %d: %s", rec.Code, rec.Body.String())
	}

	var resp ParseResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("error decodificando respuesta JSON: %v", err)
	}

	if resp.FileType != "vu" {
		t.Errorf("esperaba file_type \"vu\", obtuve %q", resp.FileType)
	}

	if resp.SignatureVerification.Status != "valid" {
		t.Errorf("esperaba signature_verification.status \"valid\", obtuve %q", resp.SignatureVerification.Status)
	}
}

func TestParseEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader([]byte{}))
	rec := httptest.NewRecorder()

	HandleParse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba HTTP 400, obtuve %d: %s", rec.Code, rec.Body.String())
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("error decodificando respuesta de error: %v", err)
	}

	if resp.Error != "empty_body" {
		t.Errorf("esperaba error \"empty_body\", obtuve %q", resp.Error)
	}
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba HTTP 200, obtuve %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("error decodificando respuesta JSON: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("esperaba status \"ok\", obtuve %q", resp["status"])
	}

	if resp["parser_version"] != ParserVersion {
		t.Errorf("esperaba parser_version %q, obtuve %q", ParserVersion, resp["parser_version"])
	}
}

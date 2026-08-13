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

	if resp.FileType != "driver_card" {
		t.Errorf("esperaba file_type \"driver_card\", obtuve %q", resp.FileType)
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

	if resp.FileType != "vehicle_unit" {
		t.Errorf("esperaba file_type \"vehicle_unit\", obtuve %q", resp.FileType)
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

// El limite debe estar alineado con el resto de la cadena (10 MB). Cuando valia
// 512 KB, un VU con velocidad detallada se rechazaba aqui aunque el frontend y
// la Edge Function ya lo hubieran aceptado.
func TestParseCuerpoDemasiadoGrande(t *testing.T) {
	grande := make([]byte, maxBodySize+1024)

	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(grande))
	rec := httptest.NewRecorder()

	HandleParse(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("esperaba HTTP 413, obtuve %d: %s", rec.Code, rec.Body.String())
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("error decodificando respuesta de error: %v", err)
	}
	if resp.Error != "body_too_large" {
		t.Errorf("esperaba error \"body_too_large\", obtuve %q", resp.Error)
	}
}

// Un archivo por debajo del limite nuevo pero por encima del viejo debe pasar
// del control de tamano (llegara al parseo y fallara alli, que es otro asunto).
func TestParseAceptaPorEncimaDelLimiteAntiguo(t *testing.T) {
	data := make([]byte, 900*1024) // 900 KB: antes se rechazaba por tamano
	data[0] = 0x76                 // cabecera de VU

	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(data))
	rec := httptest.NewRecorder()

	HandleParse(rec, req)

	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Fatal("900 KB no debe rechazarse por tamano con el limite de 10 MB")
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

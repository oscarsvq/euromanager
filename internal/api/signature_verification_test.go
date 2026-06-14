package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/traconiq/tachoparser/pkg/decoder"

	// Importar certificados para sembrar el almacen de confianza con la raiz ERCA embebida
	_ "github.com/traconiq/tachoparser/internal/pkg/certificates"
)

// parseSigStatus envia data al handler y devuelve signature_verification.status.
// Si el handler responde con error de parseo, devuelve "parse_error".
func parseSigStatus(t *testing.T, data []byte) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(data))
	rec := httptest.NewRecorder()
	HandleParse(rec, req)
	if rec.Code != http.StatusOK {
		var er ErrorResponse
		_ = json.NewDecoder(rec.Body).Decode(&er)
		return "parse_error:" + er.Error
	}
	var resp ParseResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("error decodificando respuesta: %v", err)
	}
	return resp.SignatureVerification.Status
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(testdataDir + name)
	if err != nil {
		t.Skipf("muestra no encontrada %s: %v", name, err)
	}
	return data
}

const (
	cardSample = "C_E07118343G000000_E_20260412_2105.TGD"
	vuSample   = "V_7118JST_E_20260416_1139.TGD"
)

// --- VERDADEROS POSITIVOS: las muestras reales deben verificar como "valid" ---
// Estos blindan contra una sobre-correccion que invalide archivos legitimos.

func TestSignature_GenuineCard_IsValid(t *testing.T) {
	if got := parseSigStatus(t, mustRead(t, cardSample)); got != decoder.StatusValid {
		t.Errorf("tarjeta genuina: esperaba %q, obtuve %q", decoder.StatusValid, got)
	}
}

func TestSignature_GenuineVU_IsValid(t *testing.T) {
	if got := parseSigStatus(t, mustRead(t, vuSample)); got != decoder.StatusValid {
		t.Errorf("VU genuino: esperaba %q, obtuve %q", decoder.StatusValid, got)
	}
}

// --- VERDADEROS NEGATIVOS: la manipulacion NO debe reportarse como "valid" ---
// Este es el fallo forense critico: antes del fix, cualquiera de estos devolvia "valid".

// Manipular un byte dentro de un bloque de datos firmado debe romper la firma del bloque.
func TestSignature_TamperedVUDataBlock_NotValid(t *testing.T) {
	orig := mustRead(t, vuSample)
	c := make([]byte, len(orig))
	copy(c, orig)
	// offset profundo, dentro de la actividad/eventos del VU (region firmada)
	c[40000] ^= 0xFF
	got := parseSigStatus(t, c)
	if got == decoder.StatusValid {
		t.Fatalf("VU con dato manipulado se reporto como VALID (fallo forense): %q", got)
	}
	if got != decoder.StatusInvalid && got != decoder.StatusError && got != decoder.StatusNoCert {
		t.Errorf("estado inesperado para VU manipulado: %q", got)
	}
}

func TestSignature_TamperedCardDataBlock_NotValid(t *testing.T) {
	orig := mustRead(t, cardSample)
	c := make([]byte, len(orig))
	copy(c, orig)
	c[50000] ^= 0xFF
	got := parseSigStatus(t, c)
	if got == decoder.StatusValid {
		t.Fatalf("tarjeta con dato manipulado se reporto como VALID (fallo forense): %q", got)
	}
}

// Manipular la firma del CERTIFICADO de firma del VU rompe la cadena hasta ERCA.
// Antes del fix devolvia "valid" porque .Valid del certificado nunca se leia.
func TestSignature_TamperedVUCertChain_NotValid(t *testing.T) {
	orig := mustRead(t, vuSample)
	var vu decoder.Vu
	if _, err := decoder.UnmarshalTV(orig, &vu); err != nil {
		t.Fatalf("no se pudo parsear el VU de muestra: %v", err)
	}
	certBytes := vu.SignCertificateSecondGen().Certificate
	if len(certBytes) == 0 {
		t.Skip("VU sin certificado de firma de 2a gen")
	}
	idx := bytes.Index(orig, certBytes)
	if idx < 0 {
		t.Fatal("no se localizo el certificado del VU dentro del archivo")
	}
	c := make([]byte, len(orig))
	copy(c, orig)
	// el ultimo byte del certificado forma parte de la firma ECDSA (r||s)
	c[idx+len(certBytes)-1] ^= 0xFF
	got := parseSigStatus(t, c)
	if got == decoder.StatusValid {
		t.Fatalf("VU con certificado de firma forjado se reporto como VALID (fallo forense): %q", got)
	}
	// La cadena no se puede construir -> no_cert (la CA del bloque deja de estar en el almacen).
	if got != decoder.StatusNoCert && got != decoder.StatusInvalid {
		t.Errorf("estado inesperado para certificado VU manipulado: %q", got)
	}
}

// Manipular la firma del certificado de firma de la tarjeta (gen2) rompe su cadena.
func TestSignature_TamperedCardCertChain_NotValid(t *testing.T) {
	orig := mustRead(t, cardSample)
	var card decoder.Card
	if _, err := decoder.UnmarshalTLV(orig, &card); err != nil {
		t.Fatalf("no se pudo parsear la tarjeta de muestra: %v", err)
	}
	certBytes := card.CardSignCertificate.Certificate
	if len(certBytes) == 0 {
		t.Skip("tarjeta sin certificado de firma de 2a gen")
	}
	idx := bytes.Index(orig, certBytes)
	if idx < 0 {
		t.Fatal("no se localizo el certificado de la tarjeta dentro del archivo")
	}
	c := make([]byte, len(orig))
	copy(c, orig)
	c[idx+len(certBytes)-1] ^= 0xFF
	got := parseSigStatus(t, c)
	if got == decoder.StatusValid {
		t.Fatalf("tarjeta con certificado de firma forjado se reporto como VALID (fallo forense): %q", got)
	}
}

package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/traconiq/tachoparser/pkg/decoder"
)

// ParserVersion indica la versión actual del parser
const ParserVersion = "0.1.0"

// maxBodySize es el límite de tamaño del body (512 KB)
const maxBodySize = 512 * 1024

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

// HandleParse parsea un archivo TGD (tarjeta o VU) enviado en el body
func HandleParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Limitar tamaño del body
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "body_too_large", "El archivo excede el límite de 512 KB")
		return
	}

	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "empty_body", "El body está vacío")
		return
	}

	var (
		fileType   string
		generation string
		verified   bool
		parsed     any
	)

	if data[0] == 0x76 {
		// Archivo de unidad de vehículo (VU)
		fileType = "vehicle_unit"
		generation = detectVUGeneration(data)

		var vu decoder.Vu
		verified, err = decoder.UnmarshalTV(data, &vu)
		if err != nil {
			writeError(w, http.StatusBadRequest, "parse_error", "Error parseando archivo VU: "+err.Error())
			return
		}
		parsed = vu
	} else {
		// Archivo de tarjeta de conductor
		fileType = "driver_card"
		generation = detectCardGeneration(data)

		var card decoder.Card
		verified, err = decoder.UnmarshalTLV(data, &card)
		if err != nil {
			writeError(w, http.StatusBadRequest, "parse_error", "Error parseando tarjeta: "+err.Error())
			return
		}
		parsed = card
	}

	sigStatus := "invalid"
	if verified {
		sigStatus = "valid"
	}

	resp := ParseResponse{
		ParserVersion:         ParserVersion,
		FileType:              fileType,
		Generation:            generation,
		SignatureVerification: SigResult{Status: sigStatus},
		Data:                  parsed,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// detectVUGeneration detecta la generación del archivo VU por el segundo byte
func detectVUGeneration(data []byte) string {
	if len(data) < 2 {
		return "unknown"
	}
	switch {
	case data[1] >= 0x30:
		return "gen2v2"
	case data[1] >= 0x20:
		return "gen2"
	default:
		return "gen1"
	}
}

// detectCardGeneration detecta la generación de tarjeta buscando tags gen2
// en los primeros 200 bytes (tercer byte del tag == 0x02)
func detectCardGeneration(data []byte) string {
	limit := 200
	if len(data) < limit {
		limit = len(data)
	}
	// Los tags de tarjeta tienen 3 bytes; si el tercero es 0x02 es gen2
	for i := 0; i+2 < limit; i++ {
		if data[i+2] == 0x02 {
			return "gen2"
		}
	}
	return "gen1"
}

// writeError envía una respuesta de error JSON
func writeError(w http.ResponseWriter, status int, errCode string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}

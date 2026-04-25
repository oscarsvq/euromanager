package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	// Carga las claves ERCA al inicializar
	_ "github.com/traconiq/tachoparser/internal/pkg/certificates"
	"github.com/traconiq/tachoparser/pkg/decoder"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Uso: %s <archivo.TGD>\n", os.Args[0])
		os.Exit(1)
	}

	ruta := os.Args[1]
	data, err := os.ReadFile(ruta)
	if err != nil {
		log.Fatalf("Error leyendo archivo %s: %v", ruta, err)
	}

	if len(data) == 0 {
		log.Fatalf("El archivo %s está vacío", ruta)
	}

	// Primer byte 0x76 = VU (Vehicle Unit), otro = tarjeta de conductor
	if data[0] == 0x76 {
		fmt.Fprintf(os.Stderr, "Detectado: archivo de unidad de vehículo (VU)\n")
		var vu decoder.Vu
		verified, err := decoder.UnmarshalTV(data, &vu)
		if err != nil {
			log.Fatalf("Error parseando VU: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Verificación de firma: %v\n", verified)
		imprimirJSON(vu)
	} else {
		fmt.Fprintf(os.Stderr, "Detectado: archivo de tarjeta de conductor\n")
		var card decoder.Card
		verified, err := decoder.UnmarshalTLV(data, &card)
		if err != nil {
			log.Fatalf("Error parseando tarjeta: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Verificación de firma: %v\n", verified)
		imprimirJSON(card)
	}
}

func imprimirJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Fatalf("Error serializando a JSON: %v", err)
	}
}

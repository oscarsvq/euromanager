package api

import (
	"runtime/debug"
	"strings"
)

// buildVersion lo inyecta el linker en el build:
//
//	go build -ldflags "-X github.com/traconiq/tachoparser/internal/api.buildVersion=v1.0.0-abc1234"
//
// Debe ser una var string sin inicializador: el flag -X del linker no puede
// escribir sobre una variable inicializada con una llamada a funcion.
var buildVersion string

// ParserVersion identifica la version EXACTA del binario que produjo un
// analisis. Se graba en analysis_runs.parser_version y es parte de la cadena de
// custodia: un informe pericial tiene que poder responder con que version se
// genero. Antes era la constante "0.1.0" escrita a mano, con lo que todas las
// versiones del parser resultaban indistinguibles en el audit trail.
//
// Orden de resolucion:
//  1. lo que inyecte el linker (builds de release y de la imagen Docker),
//  2. la revision de git que Go embebe automaticamente al compilar dentro de un
//     repo, marcando "+sucio" si el arbol tenia cambios sin commitear,
//  3. "desconocida" — nunca un numero de version inventado: es preferible que un
//     informe declare que no puede identificar el binario a que afirme una
//     version falsa.
var ParserVersion = resolveParserVersion()

func resolveParserVersion() string {
	if buildVersion != "" {
		return buildVersion
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "desconocida"
	}

	var revision string
	var sucio bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			sucio = s.Value == "true"
		}
	}

	if revision == "" {
		return "desconocida"
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}

	var sb strings.Builder
	sb.WriteString("dev+")
	sb.WriteString(revision)
	if sucio {
		sb.WriteString("+sucio")
	}
	return sb.String()
}

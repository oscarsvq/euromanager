package api

import (
	"strings"
	"testing"
)

// La version no puede volver a ser un literal escrito a mano: eso hacia que dos
// binarios distintos declararan lo mismo en analysis_runs.parser_version.
func TestParserVersionNoEsElLiteralAntiguo(t *testing.T) {
	if ParserVersion == "0.1.0" {
		t.Fatal("ParserVersion volvio a ser la constante fija 0.1.0; la trazabilidad del binario se pierde")
	}
	if ParserVersion == "" {
		t.Fatal("ParserVersion no puede estar vacia: se persiste en la cadena de custodia")
	}
}

// En un build local sin -ldflags la version debe identificar el commit, no
// inventarse un numero de release.
func TestParserVersionLocalIdentificaElOrigen(t *testing.T) {
	if buildVersion != "" {
		t.Skip("binario con version inyectada por el linker; este caso cubre el build local")
	}
	if !strings.HasPrefix(ParserVersion, "dev+") && ParserVersion != "desconocida" {
		t.Fatalf("esperaba una version derivada del repo o 'desconocida', obtuve %q", ParserVersion)
	}
}

func TestResolveParserVersionPrefiereElLinker(t *testing.T) {
	original := buildVersion
	t.Cleanup(func() { buildVersion = original })

	buildVersion = "v1.2.3-abc1234"
	if got := resolveParserVersion(); got != "v1.2.3-abc1234" {
		t.Fatalf("esperaba la version inyectada por el linker, obtuve %q", got)
	}
}

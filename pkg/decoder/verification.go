package decoder

// Estados forenses del resultado de verificacion de firma.
// El dominio (EuroTacho) exige distinguir cuatro estados, no solo valid/invalid:
//   - StatusValid:  la cadena completa encadeno hasta ERCA y TODOS los bloques firmados verificaron.
//   - StatusInvalid: hay al menos una firma presente que NO valida (dato manipulado o cert repudiable).
//   - StatusNoCert: no se pudo construir la cadena de confianza (falta la CA/cert para algun bloque).
//   - StatusError:  error de cripto/parseo durante la verificacion.
const (
	StatusValid   = "valid"
	StatusInvalid = "invalid"
	StatusNoCert  = "no_cert"
	StatusError   = "error"
)

// VerificationResult acumula el resultado de verificar todos los bloques firmados de un archivo.
// La agregacion es ESTRICTA: el resultado global solo es "valid" si hubo al menos un bloque
// firmado y todos verificaron, sin ningun bloque saltado por falta de cert/firma o por error.
type VerificationResult struct {
	// contadores por desenlace de cada bloque firmado encontrado
	OK      int // bloques cuya firma verifico correctamente contra un cert de confianza
	Invalid int // bloques con firma presente que NO valida (manipulacion o cert repudiable)
	NoCert  int // bloques que no se pudieron verificar por falta de cert/CA de confianza
	Errored int // bloques que produjeron un error de cripto/parseo al verificar
	// SignBlocks es el numero total de bloques firmados detectados (con tag de firma presente).
	SignBlocks int
}

// note registra el desenlace de un bloque firmado.
func (r *VerificationResult) noteOK()      { r.SignBlocks++; r.OK++ }
func (r *VerificationResult) noteInvalid() { r.SignBlocks++; r.Invalid++ }
func (r *VerificationResult) noteNoCert()  { r.SignBlocks++; r.NoCert++ }
func (r *VerificationResult) noteError()   { r.SignBlocks++; r.Errored++ }

// Status devuelve el estado forense agregado.
// Prioridad (de mas grave a menos): error > invalid > no_cert > valid.
// Un archivo sin ningun bloque firmado NO puede declararse "valid": se trata como no_cert
// (no hay nada que ancle la cadena de custodia criptografica).
func (r VerificationResult) Status() string {
	if r.SignBlocks == 0 {
		return StatusNoCert
	}
	if r.Errored > 0 {
		return StatusError
	}
	if r.Invalid > 0 {
		return StatusInvalid
	}
	if r.NoCert > 0 {
		return StatusNoCert
	}
	if r.OK > 0 && r.OK == r.SignBlocks {
		return StatusValid
	}
	// caso defensivo: no deberia alcanzarse, pero ante la duda no declaramos valido
	return StatusNoCert
}

// Verified es la version booleana estricta: true solo si Status()==valid.
func (r VerificationResult) Verified() bool {
	return r.Status() == StatusValid
}

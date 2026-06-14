package decoder

import "testing"

func TestVerificationResult_Status(t *testing.T) {
	cases := []struct {
		name string
		r    VerificationResult
		want string
	}{
		{"sin bloques firmados -> no_cert", VerificationResult{}, StatusNoCert},
		{"todos ok -> valid", VerificationResult{SignBlocks: 3, OK: 3}, StatusValid},
		{"un bloque invalido -> invalid", VerificationResult{SignBlocks: 3, OK: 2, Invalid: 1}, StatusInvalid},
		{"un bloque sin cert -> no_cert", VerificationResult{SignBlocks: 3, OK: 2, NoCert: 1}, StatusNoCert},
		{"un bloque con error -> error", VerificationResult{SignBlocks: 3, OK: 2, Errored: 1}, StatusError},
		{"error gana a invalid", VerificationResult{SignBlocks: 4, OK: 1, Invalid: 1, Errored: 1, NoCert: 1}, StatusError},
		{"invalid gana a no_cert", VerificationResult{SignBlocks: 3, OK: 1, Invalid: 1, NoCert: 1}, StatusInvalid},
		// caso forense clave: cobertura parcial (solo 1 de N bloques verifico, el resto saltados)
		// NO puede ser valid.
		{"cobertura parcial no es valid", VerificationResult{SignBlocks: 5, OK: 1, NoCert: 4}, StatusNoCert},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.r.Status()
			if got != c.want {
				t.Errorf("Status() = %q, want %q", got, c.want)
			}
			if c.r.Verified() != (c.want == StatusValid) {
				t.Errorf("Verified() = %v inconsistente con Status() = %q", c.r.Verified(), got)
			}
		})
	}
}

package api

// ParseResponse es la respuesta exitosa del endpoint /parse
type ParseResponse struct {
	ParserVersion         string    `json:"parser_version"`
	FileType              string    `json:"file_type"`
	Generation            string    `json:"generation"`
	SignatureVerification SigResult `json:"signature_verification"`
	Data                  any       `json:"data"`
}

// SigResult contiene el resultado de la verificación de firma
type SigResult struct {
	Status string `json:"status"`
}

// ErrorResponse es la respuesta de error del endpoint /parse
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

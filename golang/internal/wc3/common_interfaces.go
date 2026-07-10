package wc3

// TranslationError is reserved for future validation constraints.
type TranslationError struct {
	Message string
}

// WarResult is binary serialization output from jsonToWar.
type WarResult struct {
	Buffer []byte
	Errors []TranslationError
}

// JsonResult is parsed JSON output from warToJson.
type JsonResult[T any] struct {
	JSON   T
	Errors []TranslationError
}

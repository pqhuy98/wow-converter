package translators

import "github.com/pqhuy98/wow-converter/internal/wc3"

// Translator is the base interface for war3map chunk serializers.
type Translator[T any] interface {
	JSONToWar(json T) wc3.WarResult
	WarToJSON(buffer []byte) wc3.JsonResult[T]
}

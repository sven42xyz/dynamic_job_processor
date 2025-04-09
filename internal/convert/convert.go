package convert

import (
	"fmt"
	"strings"

	"djp.chapter42.de/a/internal/logger"
)

func MapToXML(m map[string]interface{}) []byte {
	var b strings.Builder
	for k, v := range m {
		switch v := v.(type) {
		case string:
			b.WriteString(fmt.Sprintf("<%s>%s</%s>", k, v, k))
		case float64, int, bool:
			b.WriteString(fmt.Sprintf("<%s>%v</%s>", k, v, k))
		case map[string]interface{}:
			b.WriteString(fmt.Sprintf("<%s>%s</%s>", k, MapToXML(v), k))
		case []interface{}:
			for _, item := range v {
				if m2, ok := item.(map[string]interface{}); ok {
					b.WriteString(fmt.Sprintf("<%s>%s</%s>", k, MapToXML(m2), k))
				}
			}
		default:
			logger.Log.Warn("Encountered undefined datatype in map.")
		}
	}
	return []byte(b.String())
}

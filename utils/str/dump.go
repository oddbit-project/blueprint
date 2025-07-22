package str

import (
	"encoding/json"
)

func DumpJSON(src any) string {
	result, _ := json.MarshalIndent(src, " ", " ")
	return string(result)
}

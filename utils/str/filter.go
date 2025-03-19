package str

import (
	"slices"
	"strings"
)

// Filter cuts all chars not in set, and optinally replaces them
// with the replacement rune
func Filter(s string, set []rune, replacement ...rune) string {
	if s == "" {
		return s
	}
	hasRep := len(replacement) > 0
	result := strings.Builder{}
	for _, v := range []rune(s) {
		if slices.Contains(set, v) {
			result.WriteRune(v)
		} else {
			if hasRep {
				result.WriteRune(replacement[0])
			}
		}

	}
	return result.String()
}

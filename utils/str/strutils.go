package str

// Contains returns the index of needle in haystack, if exists, or -1 if doesn't exist
func Contains(needle string, haystack []string) int {
	for i, v := range haystack {
		if v == needle {
			return i
		}
	}
	return -1
}

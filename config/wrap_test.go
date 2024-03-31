package config

import (
	"testing"
)

func TestStrOrFile(t *testing.T) {
	cases := map[string]string{
		"some value": "some value",
		"":           "",
		"the quick brown fox jumps over the lazy dog": "the quick brown fox jumps over the lazy dog",
		"./fixtures/credentials.txt":                  "the quick brown fox jumps over the lazy dog",
	}

	for k, v := range cases {
		if StrOrFile(k) != v {
			t.Error("TestStrOrFile: value mismatch", k, v)
		}
	}
}

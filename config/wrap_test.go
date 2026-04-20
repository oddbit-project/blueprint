package config

import (
	"os"
	"path/filepath"
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

func TestStrOrFileIfExists(t *testing.T) {
	tmpDir := t.TempDir()
	relativePath := filepath.Join(tmpDir, "credentials.txt")
	if err := os.WriteFile(relativePath, []byte("secret-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"some value":   "some value",
		"":             "",
		relativePath:   "secret-value",
		tmpDir:         tmpDir,
		"missing-file": "missing-file",
	}

	for input, expected := range cases {
		if got := StrOrFileIfExists(input); got != expected {
			t.Errorf("TestStrOrFileIfExists: value mismatch for %q: got %q want %q", input, got, expected)
		}
	}
}

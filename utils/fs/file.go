package fs

import (
	"os"
	"strings"
)

// FileExists checks if a given file exists (is accessible) and if it is indeed a file
// the function may fail to verify the file for other reasons, but will return false
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a given directory exists (is accessible)
// the function may fail to verify the folder for other reasons, but will return false
func DirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ReadString reads a string from a file, removing end-of-line and trimming spaces
func ReadString(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	// remove spaces and \t, \n, if present
	return strings.Trim(string(data), " \t\n"), nil

}

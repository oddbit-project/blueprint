package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "file_test_*")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tempFile.Name()) }()
	require.NoError(t, tempFile.Close())

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "file_test_dir_*")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.RemoveAll(tempDir)) }()

	// Test existing file
	assert.True(t, FileExists(tempFile.Name()))

	// Test non-existent file
	assert.False(t, FileExists(tempFile.Name()+"_nonexistent"))

	// Test directory (should return false as it's not a file)
	assert.False(t, FileExists(tempDir))
}

func TestDirExists(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "dir_test_*")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tempFile.Name()) }()
	require.NoError(t, tempFile.Close())

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "dir_test_dir_*")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.RemoveAll(tempDir)) }()

	// Test existing directory
	assert.True(t, DirExists(tempDir))

	// Test non-existent directory
	assert.False(t, DirExists(tempDir+"_nonexistent"))

	// Test file (should return false as it's not a directory)
	assert.False(t, DirExists(tempFile.Name()))
}

func TestReadString(t *testing.T) {
	// Create a temporary file with content
	content := "test content\nwith new lines  "
	expectedContent := "test content\nwith new lines"

	tempFile, err := os.CreateTemp("", "read_test_*")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tempFile.Name()) }()

	_, err = tempFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tempFile.Close())

	// Test reading existing file
	readContent, err := ReadString(tempFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, readContent)

	// Test reading non-existent file
	_, err = ReadString(tempFile.Name() + "_nonexistent")
	assert.Error(t, err)

	// Test reading directory
	tempDir, err := os.MkdirTemp("", "read_test_dir_*")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.RemoveAll(tempDir)) }()

	_, err = ReadString(tempDir)
	assert.Error(t, err)
}

func TestReadString_LineEndingsAndBOM(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"LF", "secret\n", "secret"},
		{"CRLF", "secret\r\n", "secret"},
		{"CR", "secret\r", "secret"},
		{"multiple CRLF", "secret\r\n\r\n", "secret"},
		{"BOM", "\xEF\xBB\xBFsecret", "secret"},
		{"BOM and CRLF", "\xEF\xBB\xBFsecret\r\n", "secret"},
		{"BOM only in middle kept", "sec\xEF\xBB\xBFret", "sec\xEF\xBB\xBFret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fname := filepath.Join(t.TempDir(), "secret.txt")
			require.NoError(t, os.WriteFile(fname, []byte(tt.content), 0600))

			result, err := ReadString(fname)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

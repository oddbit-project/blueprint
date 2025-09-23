package fs

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "file_test_*")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "file_test_dir_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

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
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "dir_test_dir_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

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
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(content)
	require.NoError(t, err)
	tempFile.Close()

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
	defer os.RemoveAll(tempDir)

	_, err = ReadString(tempDir)
	assert.Error(t, err)
}

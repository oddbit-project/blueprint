package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomBytes(t *testing.T) {
	// Test with normal use case
	n := uint32(32)
	bytes, err := GenerateRandomBytes(n)
	assert.NoError(t, err)
	assert.Len(t, bytes, int(n))

	// Test with zero length
	zeroBytes, err := GenerateRandomBytes(0)
	assert.NoError(t, err)
	assert.Len(t, zeroBytes, 0)

	// Test with very large length (should still work)
	largeN := uint32(1024)
	largeBytes, err := GenerateRandomBytes(largeN)
	assert.NoError(t, err)
	assert.Len(t, largeBytes, int(largeN))

	// Test randomness by comparing different results
	bytes1, _ := GenerateRandomBytes(32)
	bytes2, _ := GenerateRandomBytes(32)
	assert.NotEqual(t, bytes1, bytes2, "Two random byte arrays should not be identical")
}
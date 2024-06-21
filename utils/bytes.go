package utils

import "crypto/rand"

// GenerateRandomBytes - see crypt/hashing/argon2.go for licensing information
func GenerateRandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

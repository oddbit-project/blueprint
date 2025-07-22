package hashing

/*
 Argon2Id password hashing

  Adapted from  https://github.com/alexedwards/argon2id
  All changes are made available under the original MIT License:

	MIT License

	Copyright (c) 2018 Alex Edwards

	Permission is hereby granted, free of charge, to any person obtaining a copy
	of this software and associated documentation files (the "Software"), to deal
	in the Software without restriction, including without limitation the rights
	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
	copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all
	copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
	SOFTWARE.
*/

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"github.com/oddbit-project/blueprint/utils"
	"golang.org/x/crypto/argon2"
	"runtime"
	"strings"
)

const (
	// ErrInvalidHash in returned by ComparePasswordAndHash if the provided
	// hash isn't in the expected format.
	ErrInvalidHash = utils.Error("argon2id: hash is not in the correct format")

	// ErrIncompatibleVersion is returned by ComparePasswordAndHash if the
	// provided hash was created using a different version of Argon2.
	ErrIncompatibleVersion = utils.Error("argon2id: incompatible version of argon2")
)

// Argon2Config holds the parameters for Argon2id password hashing.
// These parameters control the computational cost and security level.
type Argon2Config struct {
	// Memory is the amount of memory to use in KiB (e.g., 64*1024 = 64MB).
	// Higher values increase security but also computational cost.
	Memory uint32 `json:"memory"`

	// Iterations is the number of passes over the memory.
	// Also known as time cost. Higher values increase security.
	Iterations uint32 `json:"iterations"`

	// Parallelism is the number of threads to use.
	// Typically set to the number of available CPU cores.
	Parallelism uint8 `json:"parallelism"`

	// SaltLength is the length of the random salt in bytes.
	// Recommended minimum is 16 bytes.
	SaltLength uint32 `json:"saltLength"`

	// KeyLength is the length of the generated hash in bytes.
	// Recommended minimum is 32 bytes.
	KeyLength uint32 `json:"keyLength"`
}

// NewArgon2IdConfig returns the default Argon2id configuration with:
//   - Memory: 64MB (64*1024 KiB)
//   - Iterations: 4
//   - Parallelism: Number of CPU cores
//   - Salt length: 16 bytes
//   - Key length: 32 bytes
func NewArgon2IdConfig() *Argon2Config {
	return &Argon2Config{
		Memory:      64 * 1024, // 64MB in KiB
		Iterations:  4,
		Parallelism: uint8(runtime.NumCPU()),
		SaltLength:  16,
		KeyLength:   32,
	}
}

// Argon2IdNeedsRehash checks if a hash was created with different
// parameters than the current default configuration. This is useful
// for upgrading hashes when security parameters are updated.
//
// Returns true if any parameter differs from the current defaults.
func Argon2IdNeedsRehash(c *Argon2Config) bool {
	cfg := NewArgon2IdConfig()
	return c.Memory != cfg.Memory || c.Iterations != cfg.Iterations || c.Parallelism != cfg.Parallelism || c.SaltLength != cfg.SaltLength || c.KeyLength != cfg.KeyLength
}

// Argon2IdCreateHash generates an Argon2id hash from the given password
// using the provided configuration. The hash is returned in the standard
// format: $argon2id$v=19$m=65536,t=4,p=8$salt$hash
//
// The function generates a cryptographically secure random salt and
// includes all parameters in the hash for self-contained verification.
func Argon2IdCreateHash(c *Argon2Config, password string) (string, error) {
	salt, err := utils.GenerateRandomBytes(c.SaltLength)
	if err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, c.Iterations, c.Memory, c.Parallelism, c.KeyLength)
	// Base64 encode the salt and hashed password.
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	// Return a string using the standard encoded hash representation.

	result := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, c.Memory, c.Iterations, c.Parallelism, b64Salt, b64Hash)
	return result, nil
}

// Argon2IdDecodeHash parses an Argon2id hash string and extracts the
// configuration parameters, salt, and hash key.
//
// The expected format is: $argon2id$v=19$m=65536,t=4,p=8$salt$hash
//
// Returns:
//   - *Argon2Config: The parameters used to create the hash
//   - []byte: The salt
//   - []byte: The hash key
//   - error: ErrInvalidHash if format is incorrect, ErrIncompatibleVersion if version mismatch
func Argon2IdDecodeHash(hash string) (*Argon2Config, []byte, []byte, error) {
	tokens := strings.Split(hash, "$")
	if len(tokens) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	// no variants allowed
	if tokens[1] != "argon2id" {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	var err error

	_, err = fmt.Sscanf(tokens[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, err
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	cfg := &Argon2Config{}
	_, err = fmt.Sscanf(tokens[3], "m=%d,t=%d,p=%d", &cfg.Memory, &cfg.Iterations, &cfg.Parallelism)
	if err != nil {
		return nil, nil, nil, err
	}

	var salt []byte
	salt, err = base64.RawStdEncoding.Strict().DecodeString(tokens[4])
	if err != nil {
		return nil, nil, nil, err
	}
	cfg.SaltLength = uint32(len(salt))

	var key []byte
	key, err = base64.RawStdEncoding.Strict().DecodeString(tokens[5])
	if err != nil {
		return nil, nil, nil, err
	}
	cfg.KeyLength = uint32(len(key))

	return cfg, salt, key, nil
}

// Argon2IdComparePassword verifies that a password matches the given Argon2id hash.
// It uses constant-time comparison to prevent timing attacks.
//
// Returns:
//   - bool: true if the password matches the hash
//   - *Argon2Config: the configuration used to create the hash (useful for rehashing)
//   - error: any error during hash parsing or comparison
//
// The configuration is returned even on failed matches to allow checking
// if rehashing is needed after a successful authentication.
//
// Example:
//
//	valid, cfg, err := Argon2IdComparePassword(password, hash)
//	if err != nil {
//	    return err
//	}
//	if valid && Argon2IdNeedsRehash(cfg) {
//	    // Generate new hash with updated parameters
//	}
func Argon2IdComparePassword(password, hash string) (bool, *Argon2Config, error) {
	cfg, salt, key, err := Argon2IdDecodeHash(hash)
	if err != nil {
		return false, nil, err
	}

	otherKey := argon2.IDKey([]byte(password), salt, cfg.Iterations, cfg.Memory, cfg.Parallelism, cfg.KeyLength)

	keyLen := int32(len(key))
	otherKeyLen := int32(len(otherKey))

	if subtle.ConstantTimeEq(keyLen, otherKeyLen) == 0 {
		return false, cfg, nil
	}
	if subtle.ConstantTimeCompare(key, otherKey) == 1 {
		return true, cfg, nil
	}
	return false, cfg, nil
}

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

// Argon2Config blueprint-style config struct
type Argon2Config struct {
	Memory      uint32 `json:"memory"`
	Iterations  uint32 `json:"iterations"`
	Parallelism uint8  `json:"parallelism"`
	SaltLength  uint32 `json:"saltLength"`
	KeyLength   uint32 `json:"keyLength"`
}

func NewArgon2IdConfig() Argon2Config {
	return Argon2Config{
		Memory:      64 * 1024, // memory in Kb
		Iterations:  4,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
}

func Argon2IdCreateHash(password string, c Argon2Config) (string, error) {
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

// Argon2ComparePassword
// Compares password and hash, and returns the hash configto enable re-hashing if necessary
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

package hashing

/*
 Argon2 password hashing

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
	"fmt"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestNewArgon2IdConfig(t *testing.T) {
	cfg := NewArgon2IdConfig()
	assert.Equal(t, uint32(64*1024), cfg.Memory)
	assert.Equal(t, uint32(4), cfg.Iterations)
	assert.Equal(t, uint8(2), cfg.Parallelism)
	assert.Equal(t, uint32(16), cfg.SaltLength)
	assert.Equal(t, uint32(32), cfg.KeyLength)
}

func TestArgon2IdCreateHash(t *testing.T) {
	hashRX, err := regexp.Compile(`^\$argon2id\$v=19\$m=65536,t=4,p=[0-9]{1,4}\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)
	assert.NoError(t, err)

	hash1, err := Argon2IdCreateHash("pa$$word", NewArgon2IdConfig())
	assert.NoError(t, err)

	assert.True(t, hashRX.MatchString(hash1), fmt.Sprintf("hash %q not in correct format", hash1))
	hash2, err := Argon2IdCreateHash("pa$$word", NewArgon2IdConfig())
	assert.NoError(t, err)
	assert.NotEqual(t, hash1, hash2, "hashes must be unique")
}

func TestArgon2IdComparePasswordAndHash(t *testing.T) {
	hash, err := Argon2IdCreateHash("pa$$word", NewArgon2IdConfig())
	assert.NoError(t, err)

	match, _, err := Argon2IdComparePassword("pa$$word", hash)
	assert.NoError(t, err)

	assert.True(t, match, "expected password and hash to match")

	match, _, err = Argon2IdComparePassword("otherPa$$word", hash)
	assert.NoError(t, err)

	assert.False(t, match, "expected password and hash to not match")
}

func TestArgon2IdDecodeHash(t *testing.T) {
	hash, err := Argon2IdCreateHash("pa$$word", NewArgon2IdConfig())
	assert.NoError(t, err)

	params, _, _, err := Argon2IdDecodeHash(hash)
	assert.NoError(t, err)

	assert.EqualExportedValues(t, NewArgon2IdConfig(), *params, fmt.Sprintf("expected %#v got %#v", NewArgon2IdConfig(), *params))
}

func TestArgon2IdCheckHash(t *testing.T) {
	hash, err := Argon2IdCreateHash("pa$$word", NewArgon2IdConfig())
	assert.NoError(t, err)

	ok, params, err := Argon2IdComparePassword("pa$$word", hash)
	assert.NoError(t, err)

	assert.True(t, ok, "expected password to match")
	assert.EqualExportedValues(t, NewArgon2IdConfig(), *params, fmt.Sprintf("expected %#v got %#v", NewArgon2IdConfig(), *params))
}

func TestArgon2IdStrictDecoding(t *testing.T) {
	// "bug" valid hash: $argon2id$v=19$m=65536,t=1,p=2$UDk0zEuIzbt0x3bwkf8Bgw$ihSfHWUJpTgDvNWiojrgcN4E0pJdUVmqCEdRZesx9tE
	ok, _, err := Argon2IdComparePassword("bug", "$argon2id$v=19$m=65536,t=1,p=2$UDk0zEuIzbt0x3bwkf8Bgw$ihSfHWUJpTgDvNWiojrgcN4E0pJdUVmqCEdRZesx9tE")
	assert.NoError(t, err)
	assert.True(t, ok, "expected password to match")

	// changed one last character of the hash
	ok, _, err = Argon2IdComparePassword("bug", "$argon2id$v=19$m=65536,t=4,p=2$UDk0zEuIzbt0x3bwkf8Bgw$ihSfHWUJpTgDvNWiojrgcN4E0pJdUVmqCEdRZesx9tF")
	assert.Error(t, err, "Hash validation should fail")
	assert.False(t, ok, "Hash validation should fail")
}

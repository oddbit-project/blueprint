package hashing

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgonHasher(t *testing.T) {
	hasher, err := NewArgon2Hasher(NewArgon2IdConfig())
	assert.Nil(t, err)
	hash, err := hasher.Generate("1231212312")
	assert.Nil(t, err)
	valid, _, err := hasher.Verify("1231212312", hash)
	assert.Nil(t, err)
	assert.True(t, valid)
}

func TestNewArgon2Hasher(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Argon2Config
		want *Argon2Config
	}{
		{
			name: "with nil config uses defaults",
			cfg:  nil,
			want: NewArgon2IdConfig(),
		},
		{
			name: "with custom config",
			cfg: &Argon2Config{
				Memory:      32 * 1024,
				Iterations:  2,
				Parallelism: 4,
				SaltLength:  32,
				KeyLength:   64,
			},
			want: &Argon2Config{
				Memory:      32 * 1024,
				Iterations:  2,
				Parallelism: 4,
				SaltLength:  32,
				KeyLength:   64,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher, err := NewArgon2Hasher(tt.cfg)
			require.NoError(t, err)
			require.NotNil(t, hasher)

			// Verify internal config by generating a hash and checking its parameters
			hash, err := hasher.Generate("test")
			require.NoError(t, err)

			cfg, _, _, err := Argon2IdDecodeHash(hash)
			require.NoError(t, err)
			assert.Equal(t, tt.want.Memory, cfg.Memory)
			assert.Equal(t, tt.want.Iterations, cfg.Iterations)
			assert.Equal(t, tt.want.SaltLength, cfg.SaltLength)
			assert.Equal(t, tt.want.KeyLength, cfg.KeyLength)
		})
	}
}

func TestPasswordHasher_Generate(t *testing.T) {
	hasher, err := NewArgon2Hasher(nil)
	require.NoError(t, err)

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "normal password",
			password: "mySecurePassword123!",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false,
		},
		{
			name:     "very long password",
			password: string(make([]byte, 1000)),
			wantErr:  false,
		},
		{
			name:     "unicode password",
			password: "пароль密码🔐",
			wantErr:  false,
		},
		{
			name:     "password with spaces",
			password: "my secure password",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := hasher.Generate(tt.password)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, hash)

			// Verify hash format
			assert.Contains(t, hash, "$argon2id$")
			assert.Contains(t, hash, "$v=19$")

			// Ensure different calls produce different hashes
			hash2, err := hasher.Generate(tt.password)
			require.NoError(t, err)
			assert.NotEqual(t, hash, hash2, "same password should produce different hashes due to random salt")
		})
	}
}

func TestPasswordHasher_Verify(t *testing.T) {
	hasher, err := NewArgon2Hasher(nil)
	require.NoError(t, err)

	password := "mySecurePassword123!"
	correctHash, err := hasher.Generate(password)
	require.NoError(t, err)

	tests := []struct {
		name       string
		password   string
		hash       string
		wantValid  bool
		wantRehash bool
		wantErr    bool
	}{
		{
			name:       "correct password",
			password:   password,
			hash:       correctHash,
			wantValid:  true,
			wantRehash: false,
			wantErr:    false,
		},
		{
			name:       "incorrect password",
			password:   "wrongPassword",
			hash:       correctHash,
			wantValid:  false,
			wantRehash: false,
			wantErr:    false,
		},
		{
			name:       "empty password with valid hash",
			password:   "",
			hash:       correctHash,
			wantValid:  false,
			wantRehash: false,
			wantErr:    false,
		},
		{
			name:       "invalid hash format",
			password:   password,
			hash:       "invalid$hash$format",
			wantValid:  false,
			wantRehash: false,
			wantErr:    true,
		},
		{
			name:       "empty hash",
			password:   password,
			hash:       "",
			wantValid:  false,
			wantRehash: false,
			wantErr:    true,
		},
		{
			name:       "corrupted base64 in hash",
			password:   password,
			hash:       "$argon2id$v=19$m=65536,t=4,p=8$invalid!base64$invalid!base64",
			wantValid:  false,
			wantRehash: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, rehashFn, err := hasher.Verify(tt.password, tt.hash)

			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, valid)
				assert.Nil(t, rehashFn)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValid, valid)

			if tt.wantRehash {
				assert.NotNil(t, rehashFn)
			} else {
				assert.Nil(t, rehashFn)
			}
		})
	}
}

func TestPasswordHasher_VerifyWithRehash(t *testing.T) {
	// Create a hasher with custom config
	customConfig := &Argon2Config{
		Memory:      32 * 1024,
		Iterations:  2,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	}

	oldHasher, err := NewArgon2Hasher(customConfig)
	require.NoError(t, err)

	password := "testPassword123!"
	oldHash, err := oldHasher.Generate(password)
	require.NoError(t, err)

	// Create a hasher with default (newer) config
	newHasher, err := NewArgon2Hasher(nil)
	require.NoError(t, err)

	// Verify with new hasher should indicate rehash is needed
	valid, rehashFn, err := newHasher.Verify(password, oldHash)
	require.NoError(t, err)
	assert.True(t, valid, "password should be valid")
	assert.NotNil(t, rehashFn, "rehash function should be provided")

	// Call rehash function
	newHash, err := rehashFn()
	require.NoError(t, err)
	assert.NotEqual(t, oldHash, newHash)

	// Verify new hash doesn't need rehashing
	valid2, rehashFn2, err := newHasher.Verify(password, newHash)
	require.NoError(t, err)
	assert.True(t, valid2)
	assert.Nil(t, rehashFn2, "new hash should not need rehashing")

	// Decode both hashes to verify parameters changed
	oldCfg, _, _, err := Argon2IdDecodeHash(oldHash)
	require.NoError(t, err)
	newCfg, _, _, err := Argon2IdDecodeHash(newHash)
	require.NoError(t, err)

	assert.Equal(t, customConfig.Memory, oldCfg.Memory)
	assert.Equal(t, uint32(64*1024), newCfg.Memory)
	assert.Equal(t, customConfig.Iterations, oldCfg.Iterations)
	assert.Equal(t, uint32(4), newCfg.Iterations)
}

func TestPasswordHasher_ConcurrentUse(t *testing.T) {
	hasher, err := NewArgon2Hasher(nil)
	require.NoError(t, err)

	password := "concurrentPassword"
	iterations := 10

	// Run multiple goroutines generating and verifying hashes
	t.Run("concurrent operations", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			t.Run("", func(t *testing.T) {
				t.Parallel()

				// Generate hash
				hash, err := hasher.Generate(password)
				require.NoError(t, err)

				// Verify hash
				valid, _, err := hasher.Verify(password, hash)
				require.NoError(t, err)
				assert.True(t, valid)
			})
		}
	})
}

func TestPasswordHasher_EdgeCases(t *testing.T) {
	hasher, err := NewArgon2Hasher(nil)
	require.NoError(t, err)

	tests := []struct {
		name     string
		password string
	}{
		{
			name:     "password with null bytes",
			password: "pass\x00word",
		},
		{
			name:     "password with newlines",
			password: "pass\nword\r\n",
		},
		{
			name:     "password with tabs",
			password: "pass\tword",
		},
		{
			name:     "all special characters",
			password: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
		{
			name:     "high unicode characters",
			password: "𝐇𝐞𝐥𝐥𝐨 𝕎𝕠𝕣𝕝𝕕",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate hash
			hash, err := hasher.Generate(tt.password)
			require.NoError(t, err)

			// Verify it
			valid, _, err := hasher.Verify(tt.password, hash)
			require.NoError(t, err)
			assert.True(t, valid)

			// Verify wrong password fails
			valid, _, err = hasher.Verify(tt.password+"wrong", hash)
			require.NoError(t, err)
			assert.False(t, valid)
		})
	}
}

func BenchmarkPasswordHasher_Generate(b *testing.B) {
	hasher, _ := NewArgon2Hasher(nil)
	password := "benchmarkPassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hasher.Generate(password)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPasswordHasher_Verify(b *testing.B) {
	hasher, _ := NewArgon2Hasher(nil)
	password := "benchmarkPassword123!"
	hash, _ := hasher.Generate(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := hasher.Verify(password, hash)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestPasswordHasher_CustomConfigNoRehash(t *testing.T) {
	// Regression test for bug where custom config hashers would incorrectly
	// trigger rehashing for their own hashes due to comparison against default config
	customConfig := &Argon2Config{
		Memory:      16 * 1024, // Different from default 64MB
		Iterations:  2,         // Different from default 4
		Parallelism: 4,         // Different from default
		SaltLength:  16,        // Same as default
		KeyLength:   32,        // Same as default
	}

	hasher, err := NewArgon2Hasher(customConfig)
	require.NoError(t, err)

	password := "testCustomConfig"
	hash, err := hasher.Generate(password)
	require.NoError(t, err)

	// Verify with same hasher - should NOT need rehashing
	valid, rehashFn, err := hasher.Verify(password, hash)
	require.NoError(t, err)
	assert.True(t, valid, "password should be valid")
	assert.Nil(t, rehashFn, "rehash function should be nil when using same custom config")

	// Verify that a default hasher WOULD trigger rehashing for the same hash
	defaultHasher, err := NewArgon2Hasher(nil)
	require.NoError(t, err)

	valid2, rehashFn2, err := defaultHasher.Verify(password, hash)
	require.NoError(t, err)
	assert.True(t, valid2, "password should be valid with different hasher")
	assert.NotNil(t, rehashFn2, "rehash function should be provided when configs differ")
}

func BenchmarkPasswordHasher_GenerateWithCustomConfig(b *testing.B) {
	// Lower memory config for faster benchmarks
	cfg := &Argon2Config{
		Memory:      16 * 1024, // 16MB
		Iterations:  2,
		Parallelism: uint8(runtime.NumCPU()),
		SaltLength:  16,
		KeyLength:   32,
	}
	hasher, _ := NewArgon2Hasher(cfg)
	password := "benchmarkPassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hasher.Generate(password)
		if err != nil {
			b.Fatal(err)
		}
	}
}

package jwtprovider

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test decodePrivateRSA
func TestDecodePrivateRSA(t *testing.T) {
	// Generate valid RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name: "valid PEM encoded private key",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privateKeyBytes,
			}),
			wantErr: nil,
		},
		{
			name: "wrong PEM type",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: privateKeyBytes,
			}),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name:    "no PEM block",
			data:    []byte("not a pem block"),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name: "invalid key data",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: []byte("invalid key data"),
			}),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name: "ECDSA key instead of RSA",
			data: func() []byte {
				ecdsaKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				ecdsaBytes, _ := x509.MarshalPKCS8PrivateKey(ecdsaKey)
				return pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: ecdsaBytes,
				})
			}(),
			wantErr: ErrInvalidPrivateKeyType,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodePrivateRSA(tt.data)
			
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				
				// Verify we can parse the result back
				parsedKey, err := x509.ParsePKCS8PrivateKey(result)
				assert.NoError(t, err)
				_, ok := parsedKey.(*rsa.PrivateKey)
				assert.True(t, ok)
			}
		})
	}
}

// Test decodePublicRSA
func TestDecodePublicRSA(t *testing.T) {
	// Generate valid RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name: "valid PEM encoded public key",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: publicKeyBytes,
			}),
			wantErr: nil,
		},
		{
			name: "wrong PEM type",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "RSA PUBLIC KEY",
				Bytes: publicKeyBytes,
			}),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name:    "no PEM block",
			data:    []byte("not a pem block"),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name: "invalid key data",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: []byte("invalid key data"),
			}),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name: "ECDSA key instead of RSA",
			data: func() []byte {
				ecdsaKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				ecdsaBytes, _ := x509.MarshalPKIXPublicKey(&ecdsaKey.PublicKey)
				return pem.EncodeToMemory(&pem.Block{
					Type:  "PUBLIC KEY",
					Bytes: ecdsaBytes,
				})
			}(),
			wantErr: ErrInvalidPublicKeyType,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodePublicRSA(tt.data)
			
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				
				// Verify we can parse the result back
				parsedKey, err := x509.ParsePKIXPublicKey(result)
				assert.NoError(t, err)
				_, ok := parsedKey.(*rsa.PublicKey)
				assert.True(t, ok)
			}
		})
	}
}

// Test decodePrivateECDSA
func TestDecodePrivateECDSA(t *testing.T) {
	// Generate valid ECDSA key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name: "valid PEM encoded private key",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privateKeyBytes,
			}),
			wantErr: nil,
		},
		{
			name: "wrong PEM type",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "EC PRIVATE KEY",
				Bytes: privateKeyBytes,
			}),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name:    "no PEM block",
			data:    []byte("not a pem block"),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name: "invalid key data",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: []byte("invalid key data"),
			}),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name: "RSA key instead of ECDSA",
			data: func() []byte {
				rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				rsaBytes, _ := x509.MarshalPKCS8PrivateKey(rsaKey)
				return pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: rsaBytes,
				})
			}(),
			wantErr: ErrInvalidPrivateKeyType,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodePrivateECDSA(tt.data)
			
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				
				// Verify we can parse the result back
				parsedKey, err := x509.ParsePKCS8PrivateKey(result)
				assert.NoError(t, err)
				_, ok := parsedKey.(*ecdsa.PrivateKey)
				assert.True(t, ok)
			}
		})
	}
}

// Test decodePublicECDSA
func TestDecodePublicECDSA(t *testing.T) {
	// Generate valid ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name: "valid PEM encoded public key",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: publicKeyBytes,
			}),
			wantErr: nil,
		},
		{
			name: "wrong PEM type",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "EC PUBLIC KEY",
				Bytes: publicKeyBytes,
			}),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name:    "no PEM block",
			data:    []byte("not a pem block"),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name: "invalid key data",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: []byte("invalid key data"),
			}),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name: "RSA key instead of ECDSA",
			data: func() []byte {
				rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				rsaBytes, _ := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
				return pem.EncodeToMemory(&pem.Block{
					Type:  "PUBLIC KEY",
					Bytes: rsaBytes,
				})
			}(),
			wantErr: ErrInvalidPublicKeyType,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodePublicECDSA(tt.data)
			
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				
				// Verify we can parse the result back
				parsedKey, err := x509.ParsePKIXPublicKey(result)
				assert.NoError(t, err)
				_, ok := parsedKey.(*ecdsa.PublicKey)
				assert.True(t, ok)
			}
		})
	}
}

// Test decodePrivateEdDSA
func TestDecodePrivateEdDSA(t *testing.T) {
	// Generate valid EdDSA key
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name: "valid PEM encoded private key",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privateKeyBytes,
			}),
			wantErr: nil,
		},
		{
			name: "wrong PEM type",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "ED25519 PRIVATE KEY",
				Bytes: privateKeyBytes,
			}),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name:    "no PEM block",
			data:    []byte("not a pem block"),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name: "invalid key data",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: []byte("invalid key data"),
			}),
			wantErr: ErrInvalidPrivateKey,
		},
		{
			name: "RSA key instead of EdDSA",
			data: func() []byte {
				rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				rsaBytes, _ := x509.MarshalPKCS8PrivateKey(rsaKey)
				return pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: rsaBytes,
				})
			}(),
			wantErr: ErrInvalidPrivateKeyType,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodePrivateEdDSA(tt.data)
			
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				
				// Verify we got an ed25519 private key (it's returned as []byte)
				assert.Len(t, result, ed25519.PrivateKeySize)
			}
		})
	}
	
	// Test that the decoded key matches the original
	t.Run("decoded key matches original", func(t *testing.T) {
		pemData := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyBytes,
		})
		
		decoded, err := decodePrivateEdDSA(pemData)
		require.NoError(t, err)
		
		// Compare with original (convert to ed25519.PrivateKey for comparison)
		decodedKey := ed25519.PrivateKey(decoded)
		assert.Equal(t, privateKey, decodedKey)
	})
	
	_ = publicKey // silence unused variable warning
}

// Test decodePublicEdDSA
func TestDecodePublicEdDSA(t *testing.T) {
	// Generate valid EdDSA key pair
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name: "valid PEM encoded public key",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: publicKeyBytes,
			}),
			wantErr: nil,
		},
		{
			name: "wrong PEM type",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "ED25519 PUBLIC KEY",
				Bytes: publicKeyBytes,
			}),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name:    "no PEM block",
			data:    []byte("not a pem block"),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name: "invalid key data",
			data: pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: []byte("invalid key data"),
			}),
			wantErr: ErrInvalidPublicKey,
		},
		{
			name: "RSA key instead of EdDSA",
			data: func() []byte {
				rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				rsaBytes, _ := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
				return pem.EncodeToMemory(&pem.Block{
					Type:  "PUBLIC KEY",
					Bytes: rsaBytes,
				})
			}(),
			wantErr: ErrInvalidPublicKeyType,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodePublicEdDSA(tt.data)
			
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				
				// Verify we got an ed25519 public key (it's returned as []byte)
				assert.Len(t, result, ed25519.PublicKeySize)
			}
		})
	}
	
	// Test that the decoded key matches the original
	t.Run("decoded key matches original", func(t *testing.T) {
		pemData := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyBytes,
		})
		
		decoded, err := decodePublicEdDSA(pemData)
		require.NoError(t, err)
		
		// Compare with original (convert to ed25519.PublicKey for comparison)
		decodedKey := ed25519.PublicKey(decoded)
		assert.Equal(t, publicKey, decodedKey)
	})
}

// Test with different elliptic curves for ECDSA
func TestDecodeECDSA_DifferentCurves(t *testing.T) {
	curves := []struct {
		name  string
		curve elliptic.Curve
	}{
		{"P224", elliptic.P224()},
		{"P256", elliptic.P256()},
		{"P384", elliptic.P384()},
		{"P521", elliptic.P521()},
	}
	
	for _, tc := range curves {
		t.Run(tc.name, func(t *testing.T) {
			// Generate key with specific curve
			privateKey, err := ecdsa.GenerateKey(tc.curve, rand.Reader)
			require.NoError(t, err)
			
			// Test private key encoding/decoding
			privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
			require.NoError(t, err)
			
			privatePEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privateKeyBytes,
			})
			
			decodedPrivate, err := decodePrivateECDSA(privatePEM)
			assert.NoError(t, err)
			assert.NotNil(t, decodedPrivate)
			
			// Test public key encoding/decoding
			publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
			require.NoError(t, err)
			
			publicPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: publicKeyBytes,
			})
			
			decodedPublic, err := decodePublicECDSA(publicPEM)
			assert.NoError(t, err)
			assert.NotNil(t, decodedPublic)
		})
	}
}

// Test error cases with malformed PEM data
func TestDecode_MalformedPEM(t *testing.T) {
	malformedData := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "incomplete PEM header",
			data: []byte("-----BEGIN PRIVATE KEY"),
		},
		{
			name: "invalid base64 in PEM",
			data: []byte(`-----BEGIN PRIVATE KEY-----
!!!invalid base64!!!
-----END PRIVATE KEY-----`),
		},
		{
			name: "missing PEM footer",
			data: []byte(`-----BEGIN PRIVATE KEY-----
SGVsbG8gV29ybGQ=
`),
		},
	}
	
	for _, tt := range malformedData {
		t.Run(tt.name, func(t *testing.T) {
			// Test all decode functions
			_, err := decodePrivateRSA(tt.data)
			assert.Error(t, err)
			
			_, err = decodePublicRSA(tt.data)
			assert.Error(t, err)
			
			_, err = decodePrivateECDSA(tt.data)
			assert.Error(t, err)
			
			_, err = decodePublicECDSA(tt.data)
			assert.Error(t, err)
			
			_, err = decodePrivateEdDSA(tt.data)
			assert.Error(t, err)
			
			_, err = decodePublicEdDSA(tt.data)
			assert.Error(t, err)
		})
	}
}
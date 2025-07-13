package jwtprovider

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

const (
	JWTIDLength = 32 // bytes of entropy
)

// GenerateSecureJWTID generates a cryptographically secure JWT ID
func GenerateSecureJWTID() (string, error) {
	// Use 32 bytes (256 bits) of entropy
	bytes := make([]byte, JWTIDLength)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure random ID: %w", err)
	}

	// Use URL-safe base64 encoding (no padding)
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func decodePrivateRSA(data []byte) ([]byte, error) {
	// Decode the PEM block
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, ErrInvalidPrivateKey
	}

	// Parse the PKCS#8 private key
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalidPrivateKey
	}

	privKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKeyType
	}
	return x509.MarshalPKCS8PrivateKey(privKey)
}

func decodePublicRSA(data []byte) ([]byte, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, ErrInvalidPublicKey
	}

	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalidPublicKey
	}

	pubKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, ErrInvalidPublicKeyType
	}
	return x509.MarshalPKIXPublicKey(pubKey)
}

func decodePrivateECDSA(data []byte) ([]byte, error) {
	// Decode the PEM block
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, ErrInvalidPrivateKey
	}

	// Parse the PKCS#8 private key
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalidPrivateKey
	}

	privKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKeyType
	}
	return x509.MarshalPKCS8PrivateKey(privKey)
}

func decodePublicECDSA(data []byte) ([]byte, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, ErrInvalidPublicKey
	}

	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalidPublicKey
	}

	pubKey, ok := pubInterface.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrInvalidPublicKeyType
	}
	return x509.MarshalPKIXPublicKey(pubKey)
}

func decodePrivateEdDSA(data []byte) ([]byte, error) {
	// Decode the PEM block
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, ErrInvalidPrivateKey
	}

	// Parse the PKCS#8 private key
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalidPrivateKey
	}

	privKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKeyType
	}
	return privKey, nil
}

func decodePublicEdDSA(data []byte) ([]byte, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, ErrInvalidPublicKey
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalidPublicKey
	}

	pubKey, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, ErrInvalidPublicKeyType
	}
	return pubKey, nil
}

func isReservedClaim(key string) bool {
	reserved := map[string]bool{
		"iss": true, "sub": true, "aud": true,
		"exp": true, "nbf": true, "iat": true,
		"jti": true, "typ": true, "alg": true,
	}
	return reserved[strings.ToLower(key)]
}

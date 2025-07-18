package jwtprovider

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	ErrInvalidSigningAlgorithm = utils.Error("JWT signing algorithm is invalid")
	ErrInvalidToken            = utils.Error("invalid token")
	ErrTokenExpired            = utils.Error("token has expired")
	ErrMissingIssuer           = utils.Error("issuer validation failed")
	ErrMissingAudience         = utils.Error("audience validation failed")
	ErrNoRevocationManager     = utils.Error("revocation manager not available")
	ErrTokenParsingTimeout     = utils.Error("token parsing timeout")
	ErrTokenTooLarge           = utils.Error("token too large")
	ErrMaxSessionsExceeded     = utils.Error("maximum concurrent sessions exceeded")

	MaxJWTLength = 8192 // 8KB max
	MinJWTLength = 20   // Minimum viable JWT
)

const (

	// Common MapClaims fields
	ClaimIssuedAt  = "iat"
	ClaimIssuer    = "iss"
	ClaimSubject   = "sub"
	ClaimAudience  = "aud"
	ClaimExpiresAt = "exp"
	ClaimNotBefore = "nbf"
	ClaimJwtID     = "jti"
)

type JWTParser interface {
	ParseToken(tokenString string) (*Claims, error)
}

type JWTSigner interface {
	GenerateToken(string, map[string]any) (string, error)
}

type JWTRefresher interface {
	Refresh(string) (string, error)
}

type JWTRevoker interface {
	RevokeToken(tokenString string) error
	RevokeTokenByID(tokenID string, expiresAt time.Time) error
	IsTokenRevoked(tokenID string) bool
}

type JWTProvider interface {
	JWTParser
	JWTSigner
	JWTRevoker
	JWTRefresher
	GetRevocationManager() *RevocationManager
	GetActiveUserTokens(userID string) ([]string, error)
	RevokeAllUserTokens(userID string) error
	GetUserSessionCount(userID string) int
}

// Claims custom claims type
type Claims struct {
	jwt.RegisteredClaims
	Data map[string]any `json:"data,omitempty"`
}

type jwtProvider struct {
	cfg               *JWTConfig
	revocationManager *RevocationManager
}

type ProviderOpts func(*jwtProvider)

func WithRevocationManager(revocationManager *RevocationManager) ProviderOpts {
	return func(p *jwtProvider) {
		p.revocationManager = revocationManager
	}
}

func NewProvider(cfg *JWTConfig, opts ...ProviderOpts) (JWTProvider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	result := &jwtProvider{
		cfg:               cfg,
		revocationManager: nil, // no revocation manager by default
	}
	for _, opt := range opts {
		opt(result)
	}
	return result, nil
}

// GenerateToken generate a JWT token using the specified alg, and optionally include the customClaims data
func (j *jwtProvider) GenerateToken(subject string, data map[string]any) (string, error) {
	// Check session limit if enabled
	if j.cfg.TrackUserTokens && j.cfg.MaxUserSessions > 0 {
		if err := j.enforceSessionLimit(subject); err != nil {
			return "", err
		}
	}

	// Prevent header injection via custom claims
	for key := range data {
		if isReservedClaim(key) {
			return "", fmt.Errorf("cannot use reserved claim: %s", key)
		}
	}
	now := time.Now()
	expiresAt := now.Add(j.cfg.expiration)

	// Generate a unique JWT ID separate from session ID for security
	jwtID, err := GenerateSecureJWTID()
	if err != nil {
		return "", err
	}

	// Create claims
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.cfg.Issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{j.cfg.Audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jwtID, // used for revocation
		},
		Data: make(map[string]any),
	}

	// copy optional data
	if data != nil {
		for k, v := range data {
			claims.Data[k] = v
		}
	}

	// Create token with key ID header for JWKS support
	token := jwt.NewWithClaims(j.cfg.signingMethod, claims)
	if j.cfg.KeyID != "" {
		token.Header["kid"] = j.cfg.KeyID
	}

	// sign token
	var tokenString string
	switch j.cfg.SigningAlgorithm {
	case HS256, HS384, HS512:
		if key, err := j.cfg.signingKey.GetBytes(); err != nil {
			return "", err
		} else {
			tokenString, err = token.SignedString(key)
			if err != nil {
				return "", err
			}
		}
	case RS256, RS384, RS512, ES256, ES384, ES512:
		if data, err := j.cfg.privateKey.GetBytes(); err == nil {
			if cert, err := x509.ParsePKCS8PrivateKey(data); err == nil {
				tokenString, err = token.SignedString(cert)
				if err != nil {
					return "", err
				}
			} else {
				return "", err
			}
		} else {
			return "", err
		}
	case EdDSA:
		// For EdDSA, convert raw bytes back to ed25519.PrivateKey
		if data, err := j.cfg.privateKey.GetBytes(); err == nil {
			// data should be raw ed25519.PrivateKey bytes (64 bytes)
			privKey := ed25519.PrivateKey(data)
			tokenString, err = token.SignedString(privKey)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	default:
		return "", ErrInvalidSigningAlgorithm
	}

	// Track the token for the user if tracking is enabled
	if j.cfg.TrackUserTokens && j.revocationManager != nil {
		if backend, ok := j.revocationManager.backend.(*MemoryRevocationBackend); ok {
			backend.TrackUserToken(subject, jwtID, expiresAt)
		}
	}

	return tokenString, nil
}

// ParseToken validates a JWT token and returns the claims
func (j *jwtProvider) ParseToken(tokenString string) (*Claims, error) {
	tokenLen := len(tokenString)
	if tokenLen < MinJWTLength {
		return nil, ErrInvalidToken
	}
	if tokenLen > j.cfg.MaxTokenSize {
		return nil, ErrTokenTooLarge
	}

	// Set parsing timeout to prevent DoS
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	var token *jwt.Token
	var err error

	// Parse token
	go func() {
		token, err = jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// ParseToken signing method
			if token.Method.Alg() != j.cfg.signingMethod.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			// Return the appropriate verification key for the algorithm
			switch j.cfg.SigningAlgorithm {
			case HS256, HS384, HS512:
				return j.cfg.signingKey.GetBytes()
			case RS256, RS384, RS512, ES256, ES384, ES512:
				bytes, err := j.cfg.publicKey.GetBytes()
				if err != nil {
					return nil, err
				}
				return x509.ParsePKIXPublicKey(bytes)
			case EdDSA:
				// For EdDSA, convert raw bytes back to ed25519.PublicKey
				if data, err := j.cfg.publicKey.GetBytes(); err == nil {
					// data should be raw ed25519.PublicKey bytes (32 bytes)
					pubKey := ed25519.PublicKey(data)
					return pubKey, nil
				} else {
					return nil, err
				}
			default:
				return nil, ErrInvalidSigningAlgorithm
			}
		})
		close(done)
	}()

	select {
	case <-done:
		break
	case <-ctx.Done():
		return nil, ErrTokenParsingTimeout
	}

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	// Get claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// Check if token is revoked
		if j.revocationManager != nil && j.revocationManager.IsTokenRevoked(claims.ID) {
			return nil, ErrTokenAlreadyRevoked
		}

		// Perform mandatory claim validation
		if err := j.validateMandatoryClaims(claims); err != nil {
			return nil, err
		}
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// validateMandatoryClaims performs mandatory issuer and audience validation
func (j *jwtProvider) validateMandatoryClaims(claims *Claims) error {
	// ParseToken issuer if required
	if j.cfg.RequireIssuer {
		if claims.Issuer == "" {
			return ErrMissingIssuer
		}
		if claims.Issuer != j.cfg.Issuer {
			return ErrMissingIssuer
		}
	}

	// ParseToken audience if required
	if j.cfg.RequireAudience {
		if len(claims.Audience) == 0 {
			return ErrMissingAudience
		}
		// Check if our audience is in the token's audience list
		audienceValid := false
		for _, aud := range claims.Audience {
			if aud == j.cfg.Audience {
				audienceValid = true
				break
			}
		}
		if !audienceValid {
			return ErrMissingAudience
		}
	}
	return nil
}

// Refresh refreshes a JWT token with token rotation
func (j *jwtProvider) Refresh(tokenString string) (string, error) {
	// ParseToken existing token
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	// Add rotation metadata to ensure token uniqueness and track rotations
	claims.Data["_rotated_at"] = time.Now().UnixNano()
	claims.Data["_rotation_count"] = getRotationCount(claims.Data) + 1

	// revoke existing token
	if err := j.RevokeToken(tokenString); err != nil {
		return "", err
	}
	
	// Generate new token with same session ID but new JWT ID
	return j.GenerateToken(claims.Subject, claims.Data)
}

// getRotationCount safely gets the rotation count from claims data
func getRotationCount(data map[string]any) int {
	if count, ok := data["_rotation_count"]; ok {
		if intCount, ok := count.(int); ok {
			return intCount
		}
		// Handle float64 from JSON unmarshaling
		if floatCount, ok := count.(float64); ok {
			return int(floatCount)
		}
	}
	return 0
}

// RevokeToken revokes a specific JWT token by extracting its ID and expiration
func (j *jwtProvider) RevokeToken(tokenString string) error {
	if j.revocationManager == nil {
		return ErrNoRevocationManager
	}
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return err
	}

	if claims.ID == "" {
		return ErrInvalidTokenID
	}

	// Use token's expiration time for revocation expiry
	var expiresAt time.Time
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	if expiresAt.IsZero() {
		// If no expiration set, use a far future date
		expiresAt = time.Now().Add(24 * time.Hour * 365)
	}

	return j.revocationManager.RevokeToken(claims.ID, expiresAt)
}

// RevokeTokenByID revokes a token by its JWT ID
func (j *jwtProvider) RevokeTokenByID(tokenID string, expiresAt time.Time) error {
	if j.revocationManager == nil {
		return ErrNoRevocationManager
	}
	return j.revocationManager.RevokeToken(tokenID, expiresAt)
}

// IsTokenRevoked checks if a token is revoked by its JWT ID
func (j *jwtProvider) IsTokenRevoked(tokenID string) bool {
	if j.revocationManager == nil {
		return false
	}
	return j.revocationManager.IsTokenRevoked(tokenID)
}

// GetRevocationManager get revocation manager instance
func (j *jwtProvider) GetRevocationManager() *RevocationManager {
	return j.revocationManager
}

// enforceSessionLimit checks and enforces session limits for a user
func (j *jwtProvider) enforceSessionLimit(userID string) error {
	if j.revocationManager == nil {
		return nil
	}

	backend, ok := j.revocationManager.backend.(*MemoryRevocationBackend)
	if !ok {
		return nil
	}

	tokens := backend.GetUserTokens(userID)
	if len(tokens) >= j.cfg.MaxUserSessions {
		return ErrMaxSessionsExceeded
	}

	return nil
}

// GetActiveUserTokens returns all active tokens for a user
func (j *jwtProvider) GetActiveUserTokens(userID string) ([]string, error) {
	if j.revocationManager == nil {
		return nil, ErrNoRevocationManager
	}

	return j.revocationManager.backend.GetUserTokens(userID), nil
}

// RevokeAllUserTokens revokes all tokens for a user
func (j *jwtProvider) RevokeAllUserTokens(userID string) error {
	if j.revocationManager == nil {
		return ErrNoRevocationManager
	}

	// Revoke all tokens issued before now
	return j.revocationManager.RevokeAllUserTokens(userID, time.Now())
}

// GetUserSessionCount returns the number of active sessions for a user
func (j *jwtProvider) GetUserSessionCount(userID string) int {
	if j.revocationManager == nil {
		return 0
	}

	tokens := j.revocationManager.backend.GetUserTokens(userID)
	return len(tokens)
}

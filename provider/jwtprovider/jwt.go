package jwtprovider

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/utils"
	"slices"
	"time"
)

const (
	ErrInvalidSigningAlgorithm = utils.Error("JWT signing algorithm is invalid")
	ErrInvalidToken            = utils.Error("invalid token")
	ErrInvalidExpClaim         = utils.Error("invalid exp claim type")
	ErrInvalidNbfClaim         = utils.Error("invalid nbf claim type")
	ErrTokenExpired            = utils.Error("token has expired")
	ErrNbfNotValid             = utils.Error("nbf not yet valid")
)

const (
	DefaultTTL = time.Minute * 30 // default duration - 30min

	// common JWT signing algorithms
	HS256 = "HS256"
	JS384 = "HS384"
	HS512 = "HS512"
	RS256 = "RS256"
	RS512 = "RS512"
	ES256 = "ES256"
	PS256 = "PS256"

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
	ParseToken(tokenString string) (jwt.MapClaims, error)
	ValidateClaims(claims jwt.MapClaims) error
}

type JWTSigner interface {
	GenerateToken(string, map[string]any) (string, error)
}

type JWTProvider interface {
	JWTParser
	JWTSigner
}

type JWTProviderOption func(*jwtProvider)

type JWTConfig struct {
	SignAlgorithms  []string `json:"signAlgorithms"`
	TokenTTLMinutes float64  `json:"tokenTTLMinutes"`
	secure.DefaultCredentialConfig
}

type jwtProvider struct {
	jwtSecret    secure.Secret
	allowedAlgos []string
	duration     time.Duration
}

func NewJWTConfig() *JWTConfig {
	return &JWTConfig{
		SignAlgorithms:  []string{HS256, JS384, HS512},
		TokenTTLMinutes: DefaultTTL.Minutes(),
	}
}

// Validate validate configuration
func (cfg *JWTConfig) Validate() error {
	if len(cfg.SignAlgorithms) == 0 {
		return errors.New("no signAlgorithms specified")
	}

	if cfg.TokenTTLMinutes < 1 {
		return errors.New("tokenTTLMinutes must be greater than zero")
	}

	return nil
}

func WithSignAlgos(signAlgos ...string) JWTProviderOption {
	return func(p *jwtProvider) {
		if len(signAlgos) > 0 {
			p.allowedAlgos = signAlgos
		}
	}
}

func WithDuration(d time.Duration) JWTProviderOption {
	return func(p *jwtProvider) {
		p.duration = d
	}
}

func NewJwtProvider(secret secure.Secret, opts ...JWTProviderOption) JWTProvider {
	if secret == nil {
		var err error
		secret, err = secure.RandomCredential(128)
		if err != nil {
			panic(err)
		}
	}

	result := &jwtProvider{
		jwtSecret:    secret,
		allowedAlgos: []string{HS256, RS256, ES256, PS256},
		duration:     DefaultTTL,
	}

	for _, o := range opts {
		o(result)
	}
	return result
}

// NewFromConfig create JWTProvider from config
func NewFromConfig(cfg *JWTConfig) (JWTProvider, error) {
	key, err := secure.GenerateKey()
	if err != nil {
		return nil, err
	}
	secret, err := secure.CredentialFromConfig(cfg.DefaultCredentialConfig, key, false)
	if err != nil {
		return nil, err
	}

	duration := time.Duration(cfg.TokenTTLMinutes) * time.Minute
	return NewJwtProvider(secret, WithSignAlgos(cfg.SignAlgorithms...), WithDuration(duration)), nil
}

// ParseToken parse JWT token
func (j *jwtProvider) ParseToken(tokenString string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(token *jwt.Token) (interface{}, error) {
			if !slices.Contains(j.allowedAlgos, token.Method.Alg()) {
				return nil, jwt.ErrSignatureInvalid
			}
			return j.jwtSecret.GetBytes(), nil
		})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GenerateToken generate a JWT token using the specified alg, and optionally include the customClaims data
func (j *jwtProvider) GenerateToken(alg string, customClaims map[string]any) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		ClaimExpiresAt: jwt.NewNumericDate(time.Now().Add(j.duration)),
		ClaimIssuedAt:  jwt.NewNumericDate(now),
		ClaimNotBefore: jwt.NewNumericDate(now),
	}
	if customClaims != nil {
		for k, v := range customClaims {
			claims[k] = v
		}
	}
	signingMethod := jwt.GetSigningMethod(alg)
	if signingMethod == nil {
		return "", ErrInvalidSigningAlgorithm
	}
	token := jwt.NewWithClaims(signingMethod, claims)

	secret, err := j.jwtSecret.GetBytes()
	if err != nil {
		return "", err
	}

	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateClaims validate time interval of the claims
func (j *jwtProvider) ValidateClaims(claims jwt.MapClaims) error {
	if err := j.ValidateExp(claims); err != nil {
		return err
	}

	return j.ValidateClaims(claims)
}

// ValidateExp validate claims expiry
func (j *jwtProvider) ValidateExp(claims jwt.MapClaims) error {
	now := time.Now().Unix()

	// Validate exp
	if expRaw, ok := claims[ClaimExpiresAt]; ok {
		exp, ok := expRaw.(float64) // JWT timestamps are float64 by default
		if !ok {
			return ErrInvalidExpClaim
		}
		if now > int64(exp) {
			return ErrTokenExpired
		}
	}

	return nil
}

// ValidateNbf validate notBefore claims
func (j *jwtProvider) ValidateNbf(claims jwt.MapClaims) error {
	now := time.Now().Unix()

	// Validate nbf
	if nbfRaw, ok := claims[ClaimNotBefore]; ok {
		nbf, ok := nbfRaw.(float64)
		if !ok {
			return ErrInvalidNbfClaim
		}
		if now < int64(nbf) {
			return ErrNbfNotValid
		}
	}

	return nil
}

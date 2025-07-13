package jwtprovider

import (
	"crypto/ecdsa"
	"crypto/x509"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	DefaultTTL      = time.Second * 86400 // 1 day
	DefaultIssuer   = "blueprint"
	DefaultAudience = "api"

	// common JWT signing algorithms
	HS256 = "HS256"
	HS384 = "HS384"
	HS512 = "HS512"
	RS256 = "RS256"
	RS384 = "RS384"
	RS512 = "RS512"
	ES256 = "ES256"
	ES384 = "ES384"
	ES512 = "ES512"
	EdDSA = "EdDSA"

	ErrSigningKeyRequired    = utils.Error("signing key is required")
	ErrPrivateKeyRequired    = utils.Error("private key is required")
	ErrPublicKeyRequired     = utils.Error("public key is required")
	ErrInvalidPrivateKey     = utils.Error("invalid private key format")
	ErrInvalidPrivateKeyType = utils.Error("invalid private key type")
	ErrInvalidPublicKey      = utils.Error("invalid public key format")
	ErrInvalidPublicKeyType  = utils.Error("invalid public key type")
	ErrInvalidDuration       = utils.Error("invalid expirationSeconds value")
	ErrInvalidMaxTokenSize   = utils.Error("invalid maxTokenSize")
)

// JWTConfig holds configuration for JWT tokens
type JWTConfig struct {
	CfgSigningKey     *secure.DefaultCredentialConfig `json:"signingKey,omitempty"`   // SigningKey is the key used to sign JWT tokens
	CfgPrivateKey     *secure.KeyConfig               `json:"privateKey,omitempty"`   // PKCS#8 private key for asymmetric algorithms (RSA, ECDSA, EdDSA)
	CfgPublicKey      *secure.KeyConfig               `json:"publicKey,omitempty"`    // PEM PKIX public key for asymmetric algorithms (RSA, ECDSA, EdDSA)
	SigningAlgorithm  string                          `json:"signingAlgorithm"`       // SigningAlgorithm: HS256/HS384/HS512, RS256/RS384/RS512, ES256/ES384/ES512, EdDSA
	ExpirationSeconds int                             `json:"expirationSeconds"`      // ExpirationSeconds
	Issuer            string                          `json:"issuer"`                 // Issuer is the issuer of the token
	Audience          string                          `json:"audience"`               // Audience is the audience of the token
	KeyID             string                          `json:"keyID"`                  // KeyID for JWKS support
	MaxTokenSize      int                             `json:"maxTokenSize,omitempty"` // Max token size
	// Enhanced validation flags
	RequireIssuer   bool `json:"requireIssuer"`   // Mandatory issuer validation
	RequireAudience bool `json:"requireAudience"` // Mandatory audience validation

	// User token tracking
	TrackUserTokens bool `json:"trackUserTokens"`           // Enable user token tracking
	MaxUserSessions int  `json:"maxUserSessions,omitempty"` // Maximum concurrent sessions per user (0 = unlimited)

	// internal vars
	signingMethod jwt.SigningMethod // signingMethod is the method used to sign the token; filled on Validate()
	expiration    time.Duration     // expiration is the expiration time for tokens; filled on Validate()
	signingKey    secure.Secret
	privateKey    secure.Secret
	publicKey     secure.Secret
	validated     bool // true if config was initialized
	// JWKS configuration
	//JWKSConfig *JWKSConfig `json:"jwksConfig,omitempty"` // JWKS endpoint configuration
}

// NewJWTConfig returns a default JWT configuration
func NewJWTConfig() *JWTConfig {
	return &JWTConfig{
		CfgSigningKey:     nil,
		CfgPrivateKey:     nil,
		CfgPublicKey:      nil,
		SigningAlgorithm:  HS256,
		signingMethod:     nil,
		ExpirationSeconds: int(DefaultTTL.Seconds()),
		expiration:        DefaultTTL,
		Issuer:            DefaultIssuer,
		Audience:          DefaultAudience,
		KeyID:             "default",
		RequireIssuer:     true, // Enable by default for security
		RequireAudience:   true, // Enable by default for security
		TrackUserTokens:   false,
		MaxUserSessions:   0, // Unlimited by default
		signingKey:        nil,
		privateKey:        nil,
		publicKey:         nil,
		validated:         false,
		MaxTokenSize:      MaxJWTLength,
	}
}

// NewJWTConfigWithKey default JWT config using a pre-defined key
func NewJWTConfigWithKey(key []byte) (*JWTConfig, error) {
	cfg := NewJWTConfig()
	var err error
	cfg.signingKey, err = secure.NewCredential(key, secure.RandomKey32(), false)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// requireSigningKey prepares internal signingKey
func (c *JWTConfig) requireSigningKey() error {
	if c.CfgSigningKey == nil && c.signingKey == nil {
		return ErrSigningKeyRequired
	}
	// if CfgSigningKey is set, override signingKey
	if c.CfgSigningKey != nil {
		if c.CfgSigningKey.IsEmpty() {
			return ErrSigningKeyRequired
		}
		var err error
		c.signingKey, err = secure.CredentialFromConfig(c.CfgSigningKey, secure.RandomKey32(), false)
		if err != nil {
			return err
		}
	}
	return nil
}

// requireRSA prepares internal public and private RSA key
func (c *JWTConfig) requireRSA() error {
	// private key
	if c.CfgPrivateKey == nil && c.privateKey == nil {
		return ErrPrivateKeyRequired
	}

	// if CfgPrivateKey is set, override privateKey
	if c.CfgPrivateKey != nil {
		if c.CfgPrivateKey.IsEmpty() {
			return ErrPrivateKeyRequired
		}

		data, err := c.CfgPrivateKey.Fetch()
		if err != nil {
			return err
		}
		key, err := decodePrivateRSA([]byte(data))
		if err != nil {
			return err
		}
		c.privateKey, err = secure.NewCredential(key, secure.RandomKey32(), false)
		if err != nil {
			return err
		}
	}

	// public key
	if c.CfgPublicKey == nil && c.publicKey == nil {
		return ErrPublicKeyRequired
	}

	// if CfgPublicKey is set, override publicKey
	if c.CfgPublicKey != nil {
		if c.CfgPublicKey.IsEmpty() {
			return ErrPublicKeyRequired
		}

		data, err := c.CfgPublicKey.Fetch()
		if err != nil {
			return err
		}
		// process cert
		key, err := decodePublicRSA([]byte(data))
		if err != nil {
			return err
		}
		c.publicKey, err = secure.NewCredential(key, secure.RandomKey32(), false)
		if err != nil {
			return err
		}
	}

	return nil
}

// requireECDSA prepares internal public and private ECDSA key
func (c *JWTConfig) requireECDSA() error {
	// private key
	if c.CfgPrivateKey == nil && c.privateKey == nil {
		return ErrPrivateKeyRequired
	}

	// if CfgPrivateKey is set, override privateKey
	if c.CfgPrivateKey != nil {
		if c.CfgPrivateKey.IsEmpty() {
			return ErrPrivateKeyRequired
		}

		data, err := c.CfgPrivateKey.Fetch()
		if err != nil {
			return err
		}
		key, err := decodePrivateECDSA([]byte(data))
		if err != nil {
			return err
		}
		c.privateKey, err = secure.NewCredential(key, secure.RandomKey32(), false)
		if err != nil {
			return err
		}
	}

	// public key
	if c.CfgPublicKey == nil && c.publicKey == nil {
		// On ECDSA, if no public key present, we can derive public key from private key
		key, err := c.privateKey.GetBytes()
		if err != nil {
			return err
		}
		privateCert, err := x509.ParsePKCS8PrivateKey(key)
		if err != nil {
			return err
		}

		data, err := x509.MarshalPKIXPublicKey(&privateCert.(*ecdsa.PrivateKey).PublicKey)
		if err != nil {
			return err
		}
		c.publicKey, err = secure.NewCredential(data, secure.RandomKey32(), false)
		return nil
	}

	// if CfgPublicKey is set, override publicKey
	if c.CfgPublicKey != nil {
		if c.CfgPublicKey.IsEmpty() {
			return ErrPublicKeyRequired
		}

		data, err := c.CfgPublicKey.Fetch()
		if err != nil {
			return err
		}
		// process cert
		key, err := decodePublicECDSA([]byte(data))
		if err != nil {
			return err
		}
		c.publicKey, err = secure.NewCredential(key, secure.RandomKey32(), false)
		if err != nil {
			return err
		}
	}

	return nil
}

// requireEdDSA prepares internal public and private EdDSA key
func (c *JWTConfig) requireEdDSA() error {
	// private key
	if c.CfgPrivateKey == nil && c.privateKey == nil {
		return ErrPrivateKeyRequired
	}

	// if CfgPrivateKey is set, override privateKey
	if c.CfgPrivateKey != nil {
		if c.CfgPrivateKey.IsEmpty() {
			return ErrPrivateKeyRequired
		}

		data, err := c.CfgPrivateKey.Fetch()
		if err != nil {
			return err
		}
		key, err := decodePrivateEdDSA([]byte(data))
		if err != nil {
			return err
		}
		c.privateKey, err = secure.NewCredential(key, secure.RandomKey32(), false)
		if err != nil {
			return err
		}
	}

	// public key
	if c.CfgPublicKey == nil && c.publicKey == nil {
		// On EdDSA, if no public key present, we can derive public key from private key
		key, err := c.privateKey.GetBytes()
		if err != nil {
			return err
		}
		c.publicKey, err = secure.NewCredential(key[32:], secure.RandomKey32(), false)
		return nil
	}

	// if CfgPublicKey is set, override publicKey
	if c.CfgPublicKey != nil {
		if c.CfgPublicKey.IsEmpty() {
			return ErrPublicKeyRequired
		}

		data, err := c.CfgPublicKey.Fetch()
		if err != nil {
			return err
		}
		// process cert
		key, err := decodePublicEdDSA([]byte(data))
		if err != nil {
			return err
		}
		c.publicKey, err = secure.NewCredential(key, secure.RandomKey32(), false)
		if err != nil {
			return err
		}
	}

	return nil
}

// Validate the JWT configuration and assemble internal fields
func (c *JWTConfig) Validate() error {
	if c.validated {
		return nil
	}

	// Get signing method
	c.signingMethod = jwt.GetSigningMethod(c.SigningAlgorithm)
	if c.signingMethod == nil {
		return ErrInvalidSigningAlgorithm
	}

	// ParseToken keys based on algorithm type
	switch c.SigningAlgorithm {
	case HS256, HS384, HS512:
		// HMAC algorithms require signing key
		if err := c.requireSigningKey(); err != nil {
			return err
		}

	case RS256, RS384, RS512:
		// RSA algorithms require private key for signing, public key for verification
		if err := c.requireRSA(); err != nil {
			return err
		}

	case ES256, ES384, ES512:
		// ECDSA algorithms require private key for signing, public key for verification
		if err := c.requireECDSA(); err != nil {
			return err
		}

	case EdDSA:
		// ECDSA algorithms require private key for signing, public key for verification
		if err := c.requireEdDSA(); err != nil {
			return err
		}
	default:
		return ErrInvalidSigningAlgorithm
	}

	if c.MaxTokenSize < 0 {
		return ErrInvalidMaxTokenSize
	}
	if c.MaxTokenSize == 0 {
		c.MaxTokenSize = MaxJWTLength
	}
	if c.ExpirationSeconds <= 0 {
		return ErrInvalidDuration
	}
	c.expiration = time.Second * time.Duration(c.ExpirationSeconds)
	c.validated = true

	return nil
}

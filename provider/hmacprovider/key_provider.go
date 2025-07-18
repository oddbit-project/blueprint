package hmacprovider

import "github.com/oddbit-project/blueprint/crypt/secure"

// HMACKeyProvider interface
// Note: userIds cannot contain dots!!! (".")
type HMACKeyProvider interface {
	FetchSecret(keyId string) (*secure.Credential, error)
}

type SingleKeyProvider struct {
	keyId  string
	secret *secure.Credential
}

// NewSingleKeyProvider creates a simple HMACKeyProvider that provides a single key
func NewSingleKeyProvider(keyId string, secret *secure.Credential) *SingleKeyProvider {
	return &SingleKeyProvider{
		keyId:  keyId,
		secret: secret,
	}
}

// FetchSecret returns the secret if the keyId is valid
func (p *SingleKeyProvider) FetchSecret(keyId string) (*secure.Credential, error) {
	if keyId == p.keyId {
		return p.secret, nil
	}
	return nil, nil
}

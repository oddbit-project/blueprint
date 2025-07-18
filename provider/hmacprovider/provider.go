package hmacprovider

import "github.com/oddbit-project/blueprint/crypt/secure"

// HMACKeyProvider interface
// Note: userIds cannot contain dots!!! (".")
type HMACKeyProvider interface {
	FetchSecret(userId string) (*secure.Credential, error)
}

type SingleKeyProvider struct {
	userId string
	secret *secure.Credential
}

// NewSingleKeyProvider creates a simple HMACKeyProvider that provides a single key
func NewSingleKeyProvider(userId string, secret *secure.Credential) *SingleKeyProvider {
	return &SingleKeyProvider{
		userId: userId,
		secret: secret,
	}
}

// FetchSecret returns the secret if the userId is valid
func (p *SingleKeyProvider) FetchSecret(userId string) (*secure.Credential, error) {
	if userId == p.userId {
		return p.secret, nil
	}
	return nil, nil
}

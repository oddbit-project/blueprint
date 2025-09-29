package kafka

import (
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/utils"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"time"
)

const (
	ErrMissingConsumerBroker    = utils.Error("Missing Consumer broker address")
	ErrMissingConsumerTopic     = utils.Error("Missing Consumer Topic or Topic Group")
	ErrConsumerAlreadyConnected = utils.Error("Cannot change connection properties; already connected")
	DefaultTimeout              = time.Second * 30

	ErrMissingProducerBroker = utils.Error("Missing Producer broker address")
	ErrMissingProducerTopic  = utils.Error("Missing Producer Topic name")
	ErrProducerClosed        = utils.Error("Producer is already closed")
	ErrInvalidAuthType       = utils.Error("Invalid authentication type")
	ErrInvalidStartOffset    = utils.Error("Invalid start offset")
	ErrInvalidIsolationLevel = utils.Error("Invalid isolation level")

	ErrMissingAdminBroker = utils.Error("Missing Admin broker address")
	ErrAdminNotConnected  = utils.Error("Admin not connected - call Connect() first")
	ErrNilConfig          = utils.Error("Config is nil")

	AuthTypeNone     = "none"
	AuthTypePlain    = "plain"
	AuthTypeScram256 = "scram256"
	AuthTypeScram512 = "scram512"
)

var validAuthTypes = []string{AuthTypeNone, AuthTypePlain, AuthTypeScram256, AuthTypeScram512}

// createSASLMechanism creates the appropriate SASL mechanism based on auth type
func createSASLMechanism(authType, username, password string) (sasl.Mechanism, error) {
	switch authType {
	case AuthTypePlain:
		return plain.Mechanism{
			Username: username,
			Password: password,
		}, nil
	case AuthTypeScram256:
		return scram.Mechanism(scram.SHA256, username, password)
	case AuthTypeScram512:
		return scram.Mechanism(scram.SHA512, username, password)
	case AuthTypeNone:
		return nil, nil
	default:
		return nil, ErrInvalidAuthType
	}
}

// setupCredentials creates and retrieves password from credential configuration
func setupCredentials(credConfig secure.DefaultCredentialConfig) (string, *secure.Credential, error) {
	key, err := secure.GenerateKey()
	if err != nil {
		return "", nil, err
	}

	credential, err := secure.CredentialFromConfig(credConfig, key, true)
	if err != nil {
		return "", nil, err
	}

	password, err := credential.Get()
	if err != nil {
		return "", nil, err
	}

	return password, credential, nil
}

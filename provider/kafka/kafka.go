package kafka

import (
	"github.com/oddbit-project/blueprint/utils"
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
	ErrNilConfig          = utils.Error("Config is nil")

	AuthTypeNone     = "none"
	AuthTypePlain    = "plain"
	AuthTypeScram256 = "scram256"
	AuthTypeScram512 = "scram512"
)

var validAuthTypes = []string{AuthTypeNone, AuthTypePlain, AuthTypeScram256, AuthTypeScram512}

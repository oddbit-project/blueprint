package kafka

import (
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	ErrMissingConsumerBroker    = utils.Error("Missing Consumer broker address")
	ErrMissingConsumerTopic     = utils.Error("Missing Consumer Topic name")
	ErrMissingConsumerGroup     = utils.Error("Missing Consumer Group")
	ErrConsumerAlreadyConnected = utils.Error("Cannot change connection properties; already connected")
	DefaultTimeout              = time.Second * 30

	ErrMissingProducerBroker = utils.Error("Missing Producer broker address")
	ErrMissingProducerTopic  = utils.Error("Missing Producer Topic name")
	ErrProducerClosed        = utils.Error("Producer is already closed")
	ErrInvalidAuthType       = utils.Error("Invalid authentication type")

	ErrMissingAdminBroker = utils.Error("Missing Admin broker address")

	AuthTypeNone     = "none"
	AuthTypePlain    = "plain"
	AuthTypeScram256 = "scram256"
	AuthTypeScram512 = "scram512"
)

var validAuthTypes = []string{AuthTypeNone, AuthTypePlain, AuthTypeScram256, AuthTypeScram512}

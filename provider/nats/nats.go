package nats

import (
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	ErrMissingConsumerURL   = utils.Error("Missing Consumer URL")
	ErrMissingConsumerTopic = utils.Error("Missing Consumer Subject")
	ErrConsumerClosed       = utils.Error("Consumer is already closed")

	ErrMissingProducerURL   = utils.Error("Missing Producer URL")
	ErrMissingProducerTopic = utils.Error("Missing Producer Subject")
	ErrProducerClosed       = utils.Error("Producer is already closed")

	ErrInvalidAuthType = utils.Error("Invalid authentication type")
	ErrNilConfig       = utils.Error("Config is nil")

	DefaultTimeout      = time.Second * 30
	DefaultConnectRetry = 3 // Number of retries to connect to NATS server

	AuthTypeNone  = "none"
	AuthTypeBasic = "basic"
	AuthTypeToken = "token"
)

var validAuthTypes = []string{AuthTypeNone, AuthTypeBasic, AuthTypeToken}

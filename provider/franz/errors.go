package franz

import "github.com/oddbit-project/blueprint/utils"

const (
	ErrNilConfig           = utils.Error("config is nil")
	ErrMissingBrokers      = utils.Error("brokers address is required")
	ErrMissingTopic        = utils.Error("topic is required")
	ErrMissingGroup        = utils.Error("consumer group is required for group consumption")
	ErrClientClosed        = utils.Error("client is closed")
	ErrInvalidAuthType     = utils.Error("invalid authentication type")
	ErrInvalidAcks         = utils.Error("invalid acks value")
	ErrInvalidCompression  = utils.Error("invalid compression type")
	ErrInvalidOffset       = utils.Error("invalid start offset value")
	ErrInvalidIsolation    = utils.Error("invalid isolation level")
	ErrTransactionAborted  = utils.Error("transaction was aborted")
	ErrNoTransactionalID   = utils.Error("transactional ID required for transactions")
	ErrNilHandler          = utils.Error("handler function is nil")
	ErrNilContext          = utils.Error("context is nil")
	ErrMissingAWSRegion    = utils.Error("AWS region is required for MSK IAM authentication")
	ErrMissingOAuthTokenURL = utils.Error("OAuth token URL is required for OAuth authentication")
)

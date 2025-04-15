package nats

import (
	"github.com/oddbit-project/blueprint/log"
)

// NatsLogContext provides context keys for NATS logging
const (
	NatsSubjectKey = "nats_subject"
	NatsQueueKey   = "nats_queue"
)

// NewProducerLogger creates a new producer logger
func NewProducerLogger(subject string) *log.Logger {
	return ProducerLogger(log.New("nats"), subject)
}

// ProducerLogger adds producer context to logger
func ProducerLogger(logger *log.Logger, subject string) *log.Logger {
	return logger.
		WithField(NatsSubjectKey, subject).
		WithField(log.LogComponentKey, "producer")
}

// NewConsumerLogger creates a new consumer logger
func NewConsumerLogger(subject string, queue string) *log.Logger {
	return ConsumerLogger(log.New("nats"), subject, queue)
}

// ConsumerLogger adds consumer context to logger
func ConsumerLogger(logger *log.Logger, subject string, queue string) *log.Logger {
	logger = logger.
		WithField(NatsSubjectKey, subject).
		WithField(log.LogComponentKey, "consumer")

	if queue != "" {
		logger = logger.WithField(NatsQueueKey, queue)
	}

	return logger
}

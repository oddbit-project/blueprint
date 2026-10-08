package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIssue99_ValidateRejectsListWithoutServers(t *testing.T) {
	for _, url := range []string{" , ", ",", " "} {
		assert.ErrorIs(t, ProducerConfig{URL: url, Subject: "s", AuthType: AuthTypeNone}.Validate(), ErrMissingProducerURL, "producer %q", url)
		assert.ErrorIs(t, ConsumerConfig{URL: url, Subject: "s", AuthType: AuthTypeNone}.Validate(), ErrMissingConsumerURL, "consumer %q", url)
		assert.ErrorIs(t, JSConnectionConfig{URL: url, AuthType: AuthTypeNone}.Validate(), ErrMissingJSURL, "jetstream %q", url)
	}
}

func TestIssue99_ValidateAcceptsListWithEmptyEntry(t *testing.T) {
	url := "nats://a:4222,, nats://b:4222"
	assert.NoError(t, ProducerConfig{URL: url, Subject: "s", AuthType: AuthTypeNone}.Validate())
	assert.NoError(t, ConsumerConfig{URL: url, Subject: "s", AuthType: AuthTypeNone}.Validate())
	assert.NoError(t, JSConnectionConfig{URL: url, AuthType: AuthTypeNone}.Validate())
}

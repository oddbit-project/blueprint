package nats

import (
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unreachableNATS refuses connections, so a client given it first must fall
// through to the next server in the list
const unreachableNATS = "nats://127.0.0.1:1"

func (s *NATSIntegrationTestSuite) TestIssue99_ProducerAndConsumerUseServerList() {
	url := unreachableNATS + "," + s.natsURL

	producer, err := NewProducer(&ProducerConfig{URL: url, Subject: "issue99.core", AuthType: AuthTypeNone}, s.logger)
	require.NoError(s.T(), err, "producer")
	defer producer.Disconnect()
	assert.True(s.T(), producer.IsConnected(), "producer")

	consumer, err := NewConsumer(&ConsumerConfig{URL: url, Subject: "issue99.core", AuthType: AuthTypeNone}, s.logger)
	require.NoError(s.T(), err, "consumer")
	defer consumer.Disconnect()
	assert.True(s.T(), consumer.IsConnected(), "consumer")
}

// nats.go shuffles the server list, so the reachable entry is tested in both positions;
// a client that kept only one entry fails one of the two orders
func (s *NATSIntegrationTestSuite) TestIssue99_ProducerUsesServerListReachableFirst() {
	url := s.natsURL + "," + unreachableNATS

	producer, err := NewProducer(&ProducerConfig{URL: url, Subject: "issue99.core", AuthType: AuthTypeNone}, s.logger)
	require.NoError(s.T(), err, "producer")
	defer producer.Disconnect()
	assert.True(s.T(), producer.IsConnected(), "producer")
}

func (s *NATSIntegrationTestSuite) TestIssue99_JetStreamUsesServerList() {
	// spaces around entries, as nats.Connect accepts
	url := " " + unreachableNATS + " , " + s.natsURL + " "
	conn := JSConnectionConfig{URL: url, AuthType: AuthTypeNone}

	producer, err := NewJSProducer(&JSProducerConfig{
		JSConnectionConfig: conn,
		Subject:            "issue99.js",
		Stream: StreamConfig{
			Name:     "ISSUE99",
			Subjects: []string{"issue99.js.>"},
			Storage:  "memory",
			Replicas: 1,
		},
		AutoCreateStream: true,
	}, s.logger)
	require.NoError(s.T(), err, "js producer")
	defer producer.Disconnect()
	assert.True(s.T(), producer.IsConnected(), "js producer")

	consumer, err := NewJSConsumer(&JSConsumerConfig{
		JSConnectionConfig: conn,
		StreamName:         "ISSUE99",
		Durable:            "issue99",
		ConsumerName:       "issue99",
		FilterSubject:      "issue99.js.>",
		AckPolicy:          "explicit",
		AckWait:            2 * time.Second,
		MaxDeliver:         5,
		MaxAckPending:      100,
		DeliverPolicy:      "all",
	}, s.logger)
	require.NoError(s.T(), err, "js consumer")
	defer consumer.Disconnect()
	assert.True(s.T(), consumer.IsConnected(), "js consumer")
}

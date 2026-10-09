package nats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

func (s *NATSIntegrationTestSuite) TestIssue114_DisconnectWaitsForAcksOnServer() {
	const n = 100
	producer := s.getTestJSProducer("TEST_JS_ISSUE114", "js.issue114")
	defer producer.Disconnect()
	futures := make([]jetstream.PubAckFuture, 0, n)
	for i := 0; i < n; i++ {
		paf, err := producer.PublishAsync(context.Background(), "js.issue114.msg", []byte("m"))
		require.NoError(s.T(), err)
		futures = append(futures, paf)
	}

	producer.Disconnect()

	for i, paf := range futures {
		select {
		case <-paf.Ok():
		case err := <-paf.Err():
			s.T().Fatalf("future %d failed: %v", i, err)
		case <-time.After(5 * time.Second):
			s.T().Fatalf("future %d unresolved 5s after Disconnect", i)
		}
	}
}

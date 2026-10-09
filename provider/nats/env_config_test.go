package nats

import (
	"testing"

	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/stretchr/testify/assert"
)

func TestIssue107_EnvProviderGetsJetStreamConfigs(t *testing.T) {
	consumer := &JSConsumerConfig{}
	stream := &StreamConfig{}
	for name, dest := range map[string]interface{}{"consumer": consumer, "stream": stream} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Get() panicked: %v", r)
				}
			}()
			assert.NoError(t, provider.NewEnvProvider("ZZ107_", false).Get(dest))
		})
	}
	assert.Nil(t, consumer.Native, "an absent Native section was allocated")
	assert.Nil(t, stream.Native, "an absent Native section was allocated")
}

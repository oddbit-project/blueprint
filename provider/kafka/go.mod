module github.com/oddbit-project/blueprint/provider/kafka

go 1.23.0

require (
	github.com/oddbit-project/blueprint v0.8.0
	github.com/segmentio/kafka-go v0.4.47
	github.com/testcontainers/testcontainers-go v0.38.0
	github.com/testcontainers/testcontainers-go/modules/kafka v0.38.0
)

replace github.com/oddbit-project/blueprint => ../../
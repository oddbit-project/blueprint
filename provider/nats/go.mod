module github.com/oddbit-project/blueprint/provider/nats

go 1.23.0

require (
	github.com/oddbit-project/blueprint v0.8.0
	github.com/nats-io/nats.go v1.41.1
	github.com/testcontainers/testcontainers-go v0.38.0
	github.com/testcontainers/testcontainers-go/modules/nats v0.38.0
)

replace github.com/oddbit-project/blueprint => ../../
# Makefile for ClickHouse integration tests

# Start Docker services
.PHONY: up
up:
	cd ../../../ && docker-compose up -d clickhouse

# Stop Docker services
.PHONY: down
down:
	cd ../../../ && docker-compose down clickhouse

# Run integration tests
.PHONY: test
test:
	go test -v -tags=integration ./...

# Run integration tests with race detection
.PHONY: test-race
test-race:
	go test -v -race -tags=integration ./...

# Start services, run tests, and stop services
.PHONY: full-test
full-test: up
	sleep 5 # Give services time to initialize
	$(MAKE) test
	$(MAKE) down
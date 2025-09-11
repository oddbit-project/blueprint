SBOM_FILE=sbom.json

.PHONY: sbom install-sbom-tool test test-unit test-integration test-all test-pgsql test-clickhouse test-kafka test-nats test-mqtt test-s3 test-db test-coverage test-coverage-unit clean-test benchmark-s3

# Run all tests (unit + integration) with race detector
test: test-all

# Run only unit tests with race detector
test-unit:
	@echo "Running unit tests with race detector..."
	@go test -v -race -short ./...

# Run all integration tests with testcontainers and race detector
test-integration:
	@echo "Running all integration tests with testcontainers and race detector..."
	@go test -v -race ./...

# Run all tests (unit + integration) with race detector
test-all:
	@echo "Running all tests (unit + integration) with race detector..."
	@go test -v -race ./...

# Individual provider tests with race detector
test-pgsql:
	@echo "Running PostgreSQL provider integration tests with testcontainers and race detector..."
	@go test -v -race ./provider/pgsql/...

test-db:
	@echo "Running database integration tests with testcontainers and race detector..."
	@go test -v -race ./db/...

test-clickhouse:
	@echo "Running ClickHouse integration tests with testcontainers and race detector..."
	@go test -v -race ./provider/clickhouse/...

test-kafka:
	@echo "Running Kafka integration tests with testcontainers and race detector..."
	@go test -v -race ./provider/kafka/...

test-nats:
	@echo "Running NATS integration tests with testcontainers and race detector..."
	@go test -v -race ./provider/nats/...

test-mqtt:
	@echo "Running MQTT integration tests with testcontainers and race detector..."
	@go test -v -race ./provider/mqtt/...

test-s3:
	@echo "Running S3 integration tests..."
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up -d minio
	@sleep 5  # Wait for MinIO to be ready
	@docker build -f Dockerfile.test.s3 -t blueprint-test-s3 .
	@docker run --rm --network blueprint_default \
		-e AWS_EC2_METADATA_DISABLED=true \
		-e S3_ENDPOINT=http://minio:9000 \
		-e S3_REGION=us-east-1 \
		-e S3_ACCESS_KEY=minioadmin \
		-e S3_SECRET_KEY=minioadmin \
		blueprint-test-s3
	@docker compose -f docker-compose.yml down -v

# Test coverage targets
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-coverage-unit:
	@echo "Running unit tests with coverage..."
	@go test -v -race -short -coverprofile=coverage_unit.out ./...
	@go tool cover -html=coverage_unit.out -o coverage_unit.html
	@echo "Unit test coverage report generated: coverage_unit.html"

# Clean test artifacts
clean-test:
	@rm -f coverage.out coverage.html coverage_unit.out coverage_unit.html
	@rm -f unit_coverage.out integration_coverage.out db_unit_coverage.html db_integration_coverage.html

benchmark-s3:
	@echo "Running S3 performance benchmarks..."
	@./scripts/run_s3_benchmark.sh

install-sbom-tool:
	go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest

sbom: install-sbom-tool
	cyclonedx-gomod mod -licenses -json -output $(SBOM_FILE)
	@echo "SBOM generated: $(SBOM_FILE)"
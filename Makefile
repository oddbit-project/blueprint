SBOM_FILE=sbom.json

# Provider modules list
PROVIDERS := kafka nats mqtt redis s3 etcd pgsql clickhouse httpserver metrics smtp htpasswd hmacprovider

.PHONY: help sbom sbom-clean install-sbom-tool test test-unit test-integration test-all test-providers test-pgsql test-clickhouse test-kafka test-nats test-mqtt test-s3 test-db test-coverage test-coverage-unit clean-test benchmark-s3
.PHONY: build build-all build-providers tidy tidy-all tidy-providers update-deps tag-version

# Default target
help:
	@echo "Blueprint Makefile Targets:"
	@echo ""
	@echo "Building:"
	@echo "  make build           - Build core Blueprint library"
	@echo "  make build-all       - Build core and all provider modules"
	@echo "  make build-providers - Build only provider modules"
	@echo ""
	@echo "Testing:"
	@echo "  make test            - Run all tests (core + providers)"
	@echo "  make test-unit       - Run only unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-providers  - Test all provider modules"
	@echo "  make test-<provider> - Test specific provider (e.g., test-kafka)"
	@echo ""
	@echo "Dependencies:"
	@echo "  make tidy            - Tidy core module dependencies"
	@echo "  make tidy-all        - Tidy all module dependencies"
	@echo "  make tidy-providers  - Tidy provider dependencies"
	@echo "  make update-deps VERSION=v0.8.0 - Update providers to Blueprint version"
	@echo ""
	@echo "Versioning:"
	@echo "  make tag-version VERSION=v0.8.0 - Tag all modules with version"
	@echo ""
	@echo "Other:"
	@echo "  make sbom            - Generate Software Bill of Materials for all modules"
	@echo "  make sbom-clean      - Clean SBOM artifacts"
	@echo "  make clean-test      - Clean test artifacts"

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
	@for provider in $(PROVIDERS); do \
		echo "Testing provider/$$provider..."; \
		(cd provider/$$provider && go test -v -race ./...) || exit 1; \
	done

# Test only provider modules
test-providers:
	@echo "Running tests for all provider modules..."
	@for provider in $(PROVIDERS); do \
		echo "Testing provider/$$provider..."; \
		(cd provider/$$provider && go test -v -race ./...) || exit 1; \
	done

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
	@echo "Generating SBOM for core module..."
	@cyclonedx-gomod mod -licenses -json -output sbom-core.json
	@mkdir -p sbom-providers
	@for provider in $(PROVIDERS); do \
		echo "Generating SBOM for provider/$$provider..."; \
		(cd provider/$$provider && cyclonedx-gomod mod -licenses -json -output ../../sbom-providers/sbom-$$provider.json) || exit 1; \
	done
	@cp sbom-core.json $(SBOM_FILE)
	@echo "Core SBOM generated: $(SBOM_FILE)"
	@echo "Provider SBOMs generated in: sbom-providers/"

sbom-clean:
	@rm -f sbom.json sbom-core.json
	@rm -rf sbom-providers/

# Build targets for modular structure
build:
	@echo "Building core Blueprint library..."
	@go build ./...

build-all: build build-providers
	@echo "All modules built successfully"

build-providers:
	@echo "Building provider modules..."
	@for provider in $(PROVIDERS); do \
		echo "Building provider/$$provider..."; \
		(cd provider/$$provider && go build ./...) || exit 1; \
	done

# Dependency management for modular structure
tidy:
	@echo "Tidying core module dependencies..."
	@go mod tidy

tidy-all: tidy tidy-providers
	@echo "All module dependencies tidied"

tidy-providers:
	@echo "Tidying provider module dependencies..."
	@for provider in $(PROVIDERS); do \
		echo "Tidying provider/$$provider..."; \
		(cd provider/$$provider && go mod tidy) || exit 1; \
	done

# Update all provider dependencies to latest Blueprint version
update-deps:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make update-deps VERSION=v0.8.0"; \
		exit 1; \
	fi
	@echo "Updating all provider modules to Blueprint $(VERSION)..."
	@for provider in $(PROVIDERS); do \
		echo "Updating provider/$$provider..."; \
		(cd provider/$$provider && go get github.com/oddbit-project/blueprint@$(VERSION) && go mod tidy) || exit 1; \
	done

# Tag all modules with a version
tag-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make tag-version VERSION=v0.8.0"; \
		exit 1; \
	fi
	@echo "Tagging Blueprint core as $(VERSION)..."
	@git tag $(VERSION) 2>/dev/null || echo "Core tag $(VERSION) already exists"
	@echo "Tagging all provider modules as $(VERSION)..."
	@for provider in $(PROVIDERS); do \
		echo "Tagging provider/$$provider..."; \
		git tag provider/$$provider/$(VERSION) 2>/dev/null || echo "Tag provider/$$provider/$(VERSION) already exists"; \
	done
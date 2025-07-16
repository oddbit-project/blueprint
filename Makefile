SBOM_FILE=sbom.json

.PHONY: sbom install-sbom-tool test test-pgsql test-clickhouse test-kafka test-nats test-mqtt

test:
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up --build --abort-on-container-exit --remove-orphans --force-recreate
	@docker compose -f docker-compose.yml down -v

test-pgsql:
	@echo "Running PostgreSQL integration tests..."
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up -d postgres
	@docker build -f Dockerfile.test.pgsql -t blueprint-test-pgsql .
	@docker run --rm --network blueprint_default \
		-e POSTGRES_HOST=postgres \
		-e POSTGRES_PORT=5432 \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=password \
		-e POSTGRES_DB=postgres \
		blueprint-test-pgsql
	@docker compose -f docker-compose.yml down -v

test-clickhouse:
	@echo "Running ClickHouse integration tests..."
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up -d clickhouse
	@docker build -f Dockerfile.test.clickhouse -t blueprint-test-clickhouse .
	@docker run --rm --network blueprint_default \
		-e CLICKHOUSE_HOST=clickhouse \
		-e CLICKHOUSE_PORT=9000 \
		-e CLICKHOUSE_USER=default \
		-e CLICKHOUSE_PASSWORD= \
		-e CLICKHOUSE_DB=default \
		blueprint-test-clickhouse
	@docker compose -f docker-compose.yml down -v

test-kafka:
	@echo "Running Kafka integration tests..."
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up -d zookeeper kafka
	@sleep 10  # Wait for Kafka to be ready
	@docker build -f Dockerfile.test.kafka -t blueprint-test-kafka .
	@docker run --rm --network blueprint_default blueprint-test-kafka
	@docker compose -f docker-compose.yml down -v

test-nats:
	@echo "Running NATS integration tests..."
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up -d nats
	@docker build -f Dockerfile.test.nats -t blueprint-test-nats .
	@docker run --rm --network blueprint_default \
		-e NATS_SERVER_HOST=nats \
		blueprint-test-nats
	@docker compose -f docker-compose.yml down -v

test-mqtt:
	@echo "Running MQTT integration tests..."
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up -d mosquitto
	@docker build -f Dockerfile.test.mqtt -t blueprint-test-mqtt .
	@docker run --rm --network blueprint_default blueprint-test-mqtt
	@docker compose -f docker-compose.yml down -v


install-sbom-tool:
	go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest

sbom: install-sbom-tool
	cyclonedx-gomod mod -licenses -json -output $(SBOM_FILE)
	@echo "SBOM generated: $(SBOM_FILE)"
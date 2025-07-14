SBOM_FILE=sbom.json

.PHONY: sbom install-sbom-tool test

test:
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up --build --abort-on-container-exit --remove-orphans --force-recreate
	@docker compose -f docker-compose.yml down -v


install-sbom-tool:
	go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest

sbom: install-sbom-tool
	cyclonedx-gomod mod -licenses -json -output $(SBOM_FILE)
	@echo "SBOM generated: $(SBOM_FILE)"
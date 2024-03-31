.PHONY: test
test:
	@docker compose -f docker-compose.yml down -v
	@docker compose -f docker-compose.yml up --build --abort-on-container-exit --remove-orphans --force-recreate
	@docker compose -f docker-compose.yml down -v

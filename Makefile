.PHONY: RUN
run:
	@./scripts/run.sh

.PHONY: lint
lint:
	@golangci-lint run --fix

.PHONY: tests
tests:
	@go test ./...

.PHONY: install-dependencies
install-dependencies:
	@go get ./...

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: prepare-commit
prepare-commit: lint tests
	@echo lint and testing passed

.PHONY: db-up
db-up:
	@docker-compose -f docker-compose.yml up -d postgres

.PHONY: down-deps
down-deps:
	@docker-compose -f docker-compose.yml down

.PHONY: up-deps
up-deps:
	@docker-compose -f ./deploy/docker-compose.yml up -d postgres redis

.PHONY: build-docker
build-docker:
	@./scripts/build-image.sh

.PHONY: compose-up
compose-up:
	@./scripts/build-image.sh
	@docker-compose -f ./deploy/docker-compose.yml up -d

.PHONY: compose-complete-down
compose-down:
	@docker-compose -f ./deploy/docker-compose.yml down
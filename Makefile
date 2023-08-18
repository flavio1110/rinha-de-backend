.PHONY: RUN
run:
	@go run .

.PHONY: lint
lint:
	@gofmt -w .

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

.PHONY: compose-up
compose-up:
	@docker-compose -f docker-compose.yml up -d

.PHONY: compose-down
compose-down:
	@docker-compose -f docker-compose.yml down

.PHONY: up-deps
up-local-dependencies:
	@docker-compose -f docker-compose.yml up -d postgres




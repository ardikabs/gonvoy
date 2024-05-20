SHELL				:= /bin/bash
GOLANGCI_VERSION	= 1.57.2
GOLANG_FILES		= $(shell go list ./... | grep -vE '/vendor|/mock'|xargs echo)

.PHONY: help
help: ## Print this help message.
	@echo -e "Usage:\n  make \033[36m[Target]\033[0m\n\nTargets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} \
			/^[a-zA-Z0-9.]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } \
			/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' \
			$(MAKEFILE_LIST)

.PHONY: audit
audit: fmt mod lint vet test ## Doing full audit to the code such as formatting, tidying, linting, venting, and testing code.

.PHONY: fmt
fmt: ## Formatting the code.
	@echo 'Formatting code ...'
	@go fmt $(GOLANG_FILES)

.PHONY: mod
mod: ## Tidying code dependencies.
	@echo 'Tidying, verifying, and vendoring module dependencies ...'
	@go mod tidy
	@go mod verify
	@go mod vendor

.PHONY: vet
vet: ## Vetting the code.
	@echo 'Vetting code ...'
	@go vet $(GOLANG_FILES)

.PHONY: test
test: ## Run unit test for the code.
	@echo 'Running tests ...'
	@mkdir -p output
	@go test $(GOLANG_FILES) -cover -coverprofile=./output/coverage.out -race && \
		go tool cover -html=./output/coverage.out -o ./output/coverage.html && \
		go tool cover -func=./output/coverage.out

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION}
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint

bin/golangci-lint-${GOLANGCI_VERSION}:
	@mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b bin v$(GOLANGCI_VERSION)
	@mv bin/golangci-lint "$@"

.PHONY: lint
lint: bin/golangci-lint # Linting the code with golangci-lint.
	@echo 'Linting code ...'
	bin/golangci-lint run

.PHONY: e2e
e2e: e2e.build e2e.go ## Run e2e tests.

.PHONY: e2e.go
e2e.go: ## Run direct e2e tests without building filters.
	@go test -v -tags=e2e ./test/e2e

.PHONY: e2e.build
e2e.build: ## Build filters for e2e tests.
	@docker compose -f docker-compose-e2e.yaml run --rm --build e2e_filters_compile

.PHONY: e2e.cleanup
e2e.cleanup: ## Clean up e2e filters.
	@./hack/cleanup-e2e.sh

.PHONE: run.example
run.example:
	@docker compose up --build --remove-orphans
GOLANGCI_VERSION = 1.53.3
SHELL:=/bin/bash

.PHONY: help
help: ## Print this help message.
	@echo -e "Usage:\n  make \033[36m[Target]\033[0m\n\nTargets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} \
			/^[a-zA-Z.]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } \
			/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' \
			$(MAKEFILE_LIST)

.PHONY: audit
audit: fmt mod lint vet test ## Doing full audit to the code such as formatting, tidying, linting, venting, and testing code.

.PHONY: fmt
fmt: ## Formatting the code.
	@echo 'Formatting code ...'
	@go fmt $(shell go list ./... | grep -v /vendor/|xargs echo)

.PHONY: mod
mod: ## Tidying code dependencies.
	@echo 'Tidying, verifying, and vendoring module dependencies ...'
	@go mod tidy
	@go mod verify
	@go mod vendor

.PHONY: vet
vet: ## Vetting the code.
	@echo 'Vetting code ...'
	@go vet $(shell go list ./... | grep -v /vendor/|xargs echo)

.PHONY: test
test: ## Run unit test for the code.
	@echo 'Running tests ...'
	@mkdir -p output
	@go test $(shell go list ./... | grep -v /vendor/|xargs echo) -cover -coverprofile=./output/coverage.out -race && \
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
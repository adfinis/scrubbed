.DEFAULT_GOAL := help

.PHONY: help
# Self documenting Makefile
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

GO_LINT=$(shell which golangci-lint 2> /dev/null || echo '')
GO_LINT_URI=github.com/golangci/golangci-lint/cmd/golangci-lint@latest

GO_SEC=$(shell which gosec 2> /dev/null || echo '')
GO_SEC_URI=github.com/securego/gosec/v2/cmd/gosec@latest

GO_VULNCHECK=$(shell which govulncheck 2> /dev/null || echo '')
GO_VULNCHECK_URI=golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint
	$(if $(GO_LINT), ,go install $(GO_LINT_URI))
	@echo "##### Running golangci-lint"
	golangci-lint run -v
	
.PHONY: gosec
gosec: ## Run gosec
	$(if $(GO_SEC), ,go install $(GO_SEC_URI))
	@echo "##### Running gosec"
	gosec ./...

.PHONY: govulncheck
govulncheck: ## Run govulncheck
	$(if $(GO_VULNCHECK), ,go install $(GO_VULNCHECK_URI))
	@echo "##### Running govulncheck"
	govulncheck ./...

.PHONY: verify
verify: golangci-lint gosec govulncheck ## Run all checks

.PHONY: test
test: ## Run Go tests
	@echo "##### Running tests"
	go test -race -cover -coverprofile=coverage.coverprofile -covermode=atomic -v ./...

.PHONY: tidy
tidy: ## Tidy go.mod
	go mod tidy

.PHONY: build
build: ## Build scrubbed
	go build

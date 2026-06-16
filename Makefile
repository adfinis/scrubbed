.DEFAULT_GOAL := help

.PHONY: help
# Self documenting Makefile
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

GO_LINT=$(shell which golangci-lint 2> /dev/null || echo '')
GO_LINT_VERSION=v2.12.2

GO_SEC=$(shell which gosec 2> /dev/null || echo '')
GO_SEC_URI=github.com/securego/gosec/v2/cmd/gosec@v2.27.1

GO_VULNCHECK=$(shell which govulncheck 2> /dev/null || echo '')
GO_VULNCHECK_URI=golang.org/x/vuln/cmd/govulncheck@v1.3.0

.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint
	$(if $(GO_LINT), ,curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(GOPATH)/bin $(GO_LINT_VERSION))
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
	CGO_ENABLED=0 go build

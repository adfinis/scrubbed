.DEFAULT_GOAL := help

.PHONY: help
# Self documenting Makefile
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'


# -t $(IMAGE_NAME):$(VERSION) .

.PHONY: image
image: ## Create Docker image
	podman build .
	@echo built image $(IMAGE_NAME)

.PHONY: venv
venv: ## Initialize virtual environment and install dependencies
	./initenv.sh

.PHONY: static
static: venv ## Generate static binary with embedded Python
	venv/bin/pyinstaller --onefile scrubbed.py

.PHONY: clean
clean: ## Clean up
	rm -rf venv/ build/ dist/ __pycache__/ scrubbed.spec
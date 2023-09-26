BINARY     := http2socks
BUILDFLAGS := "-s -w"

.SUFFIXES:
.PHONY: build lint help
.DEFAULT_GOAL := help

### MAIN COMMANDS ###

build: lint ## Build binary
	@go mod tidy -e
	@mkdir -p build
	go build -ldflags $(BUILDFLAGS) -o build/$(BINARY) *.go

### ADDITIONAL COMMANDS ###

lint: ## Run linter
	golangci-lint run

# Auto documented Makefile https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

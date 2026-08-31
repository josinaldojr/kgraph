BINARY_NAME := kgraph
CMD_DIR      := ./cmd/kgraph
BUILD_DIR    := ./bin
EXE_EXT      := $(shell go env GOEXE)
BINARY       := $(BUILD_DIR)/$(BINARY_NAME)$(EXE_EXT)

GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

.PHONY: all build install uninstall run test fmt vet tidy clean help

all: build

build: ## Build the kgraph binary into ./bin
	go build -o $(BINARY) $(CMD_DIR)

install: ## Build and install kgraph onto GOPATH/GOBIN (adds it to your PATH once GOBIN is on it)
	go install $(CMD_DIR)
	@echo "Installed to $(GOBIN)/$(BINARY_NAME)$(EXE_EXT)"

uninstall: ## Remove the globally installed kgraph binary
	rm -f "$(GOBIN)/$(BINARY_NAME)$(EXE_EXT)"

run: build ## Build and run kgraph (use ARGS="..." to pass arguments)
	$(BINARY) $(ARGS)

test: ## Run the test suite
	go test ./...

fmt: ## Format all Go source files
	go fmt ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod/go.sum
	go mod tidy

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

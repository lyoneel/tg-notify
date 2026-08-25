SHELL := /bin/bash

# Project settings
NAME     := tg-notify
MAIN_PKG := ./cmd/tg-notify
BINARY   := build/$(NAME)
ARGS     ?=
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Tools
GO     := go
GOFMT  := gofmt
GOLINT := golangci-lint

.PHONY: help build release release-all _release run test test-race fmt vet lint audit vuln bench deadcode generate clean

help: ## Show this help
	@echo "$(NAME)"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?##"} { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }'

# --- Build ---

build: ## Build the binary
	@mkdir -p build
	@$(GO) build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) $(MAIN_PKG)

release: ## Build release binaries (linux, darwin, windows, Pi arm)
	@$(MAKE) --no-print-directory _release TARGETS="linux/amd64 darwin/amd64 darwin/arm64 windows/amd64 linux/arm/6 linux/arm/7"

release-all: ## Build release binaries for every supported platform
	@$(MAKE) --no-print-directory _release TARGETS="linux/amd64 linux/386 linux/arm64 linux/arm/5 linux/arm/6 linux/arm/7 linux/riscv64 darwin/amd64 darwin/arm64 windows/amd64 windows/386 windows/arm64 freebsd/amd64 freebsd/386 freebsd/arm64 freebsd/arm/7 freebsd/riscv64 openbsd/amd64 openbsd/386 openbsd/arm64 openbsd/arm/7"

_release:
	@mkdir -p build
	@set -e; \
	for t in $(TARGETS); do \
		os=$${t%%/*}; rest=$${t#*/}; arch=$${rest%%/*}; \
		arm=$${rest##*/}; [ "$$arm" = "$$rest" ] && arm=; \
		suffix=$$os-$$arch; [ -n "$$arm" ] && suffix=$$os-$$arch-v$$arm; \
		ext=; [ "$$os" = windows ] && ext=.exe; \
		out=build/$(NAME)-$$suffix$$ext; \
		echo "  $$os/$$arch$${arm:+/v$$arm} -> $$out"; \
		GOOS=$$os GOARCH=$$arch GOARM=$$arm CGO_ENABLED=0 \
			$(GO) build -ldflags "-X main.version=$(VERSION)" -o "$$out" $(MAIN_PKG); \
	done

# --- Run ---

run: build ## Run the CLI; pass ARGS="message"
	@$(BINARY) $(ARGS)

# --- Quality ---

test: ## Run all tests
	@$(GO) test ./...

test-race: ## Run tests with the race detector
	@$(GO) test -race ./...

fmt: ## Format Go code
	@$(GOFMT) -w .

vet: ## Run go vet
	@$(GO) vet ./...

lint: ## Run golangci-lint
	@command -v $(GOLINT) > /dev/null 2>&1 || { echo "Install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"; exit 1; }
	@$(GOLINT) run ./...

audit: test vet lint ## Run all quality checks; adds govulncheck when online
	@test -z "$$($(GOFMT) -l .)" || { echo "gofmt: files need formatting (run make fmt)"; exit 1; }
	@$(GO) mod verify
	@$(GO) mod tidy -diff
	@if timeout 3 bash -c ': </dev/tcp/proxy.golang.org/443' 2>/dev/null; then \
		echo "online: checking dependencies for vulnerabilities"; \
		$(MAKE) --no-print-directory vuln; \
	else \
		echo "offline: skipping govulncheck (run 'make vuln' when online)"; \
	fi

vuln: ## Check dependencies for vulnerabilities
	@command -v govulncheck > /dev/null 2>&1 || { echo "Install: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; }
	@govulncheck ./...

bench: ## Run benchmarks (compare runs with benchstat)
	@$(GO) test -run=^$ -bench=. -benchmem ./...

deadcode: ## Report unreachable exported code
	@command -v deadcode > /dev/null 2>&1 || { echo "Install: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; }
	@deadcode ./...

# --- Codegen ---

generate: ## Run go generate directives (stringer, protoc, ...)
	@$(GO) generate ./...

# --- Clean ---

clean: ## Remove build artifacts (keeps build/ and its .gitkeep)
	@test ! -d build || find build -mindepth 1 -maxdepth 1 ! -name .gitkeep -exec rm -rf {} +
	@$(GO) clean

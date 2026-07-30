BINARY := karya
PKG := github.com/drjzlyan/karya
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.Date=$(DATE)

# golangci-lint version: match CI, which installs `latest`. Override to pin.
GOLANGCI_VERSION ?= latest

.PHONY: build install fmt vet lint test tidy clean run sync-docs formula gate

build: ## Build the karya binary into ./bin
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

sync-docs: ## Vendor docs/*.md into internal/assets/docs for embedding
	./scripts/sync-docs.sh

formula: ## Regenerate the Homebrew formula (usage: make formula TAG=v0.1.0 SUMS=dist/checksums.txt)
	./scripts/update-formula.sh "$(TAG)" "$(SUMS)"

install: ## Install karya into ~/.local/bin
	go build -ldflags "$(LDFLAGS)" -o $$HOME/.local/bin/$(BINARY) .

run: build ## Build and run
	./bin/$(BINARY)

fmt: ## Format sources
	gofmt -s -w .

vet: ## Static analysis
	go vet ./...

lint: ## Run golangci-lint (matches CI: golangci-lint v2 @ $(GOLANGCI_VERSION))
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...

test: ## Run tests
	go test ./...

gate: ## Full pre-PR gate (mirrors CI): fmt-check, vet, lint, race + integration tests, build
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
	$(MAKE) lint
	go test -race ./...
	go test -tags=integration ./...
	go build ./...

tidy: ## Tidy modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin

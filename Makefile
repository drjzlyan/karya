BINARY := karya
PKG := github.com/drjzlyan/karya
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.Date=$(DATE)

.PHONY: build install fmt vet test tidy clean run sync-nvim

build: ## Build the karya binary into ./bin
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

sync-nvim: ## Vendor ../nvim-config into internal/assets/nvim for embedding
	./scripts/sync-nvim.sh

install: ## Install karya into ~/.local/bin
	go build -ldflags "$(LDFLAGS)" -o $$HOME/.local/bin/$(BINARY) .

run: build ## Build and run
	./bin/$(BINARY)

fmt: ## Format sources
	gofmt -s -w .

vet: ## Static analysis
	go vet ./...

test: ## Run tests
	go test ./...

tidy: ## Tidy modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin

BINARY  := tasker
PKG     := github.com/mahm0udnasr/tasker
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/cmd.Version=$(VERSION)
GOBIN   ?= $(shell go env GOPATH)/bin

export CGO_ENABLED := 0

.PHONY: all build run test test-race cover lint fmt tidy check-tidy install uninstall snapshot demo screenshots clean help

all: fmt check-tidy lint test build ## Format, tidy-check, lint, test and build

build: ## Build ./bin/tasker
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) .

run: build ## Build and launch tasker (pass ARGS="list --today")
	./bin/$(BINARY) $(ARGS)

test: ## Run tests
	go test -count=1 ./...

test-race: ## Run tests with the race detector (needs a C toolchain)
	CGO_ENABLED=1 go test -race -count=1 ./...

cover: ## Write an HTML coverage report to coverage.html
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format code (gofmt + goimports)
	golangci-lint fmt ./... 2>/dev/null || gofmt -s -w .

tidy: ## Tidy go.mod
	go mod tidy

check-tidy: ## Fail if go.mod/go.sum need tidying (as CI does); changes nothing
	go mod tidy -diff

install: ## Install to $GOBIN (default ~/go/bin)
	go install -trimpath -ldflags '$(LDFLAGS)' .

uninstall: ## Remove the installed binary
	rm -f $(GOBIN)/$(BINARY)

snapshot: ## Build release archives and .deb/.rpm locally into dist/ (needs goreleaser)
	goreleaser release --snapshot --clean

demo: build ## Record docs/demo.gif (needs vhs)
	PATH="$(CURDIR)/bin:$$PATH" vhs docs/demo.tape

screenshots: ## Regenerate docs/screenshots from the TUI (needs python3 + Pillow)
	@mkdir -p .shots
	TASKER_SNAPSHOT_DIR=$(CURDIR)/.shots go test -count=1 -run TestScreenshots ./internal/tui/
	for n in main focus stats; do python3 docs/screenshots/render.py .shots/shot-$$n.ansi docs/screenshots/$$n.png 1e1e2e cdd6f4; done
	python3 docs/screenshots/render.py .shots/shot-themes.ansi docs/screenshots/themes.png 2e3440 eceff4
	rm -rf .shots

clean: ## Remove build artifacts
	rm -rf bin dist coverage.out coverage.html

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

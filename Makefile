# Build, install and check floe from source. Go stamps the version from
# git, so a build here reports the commit it was made from.

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: build install uninstall test test-short check fmt-check vet unit e2e snapshot clean help

build: ## Build ./floe
	go build -trimpath -o floe ./cmd/floe

# install builds only when nothing is built yet, so "make && sudo make
# install" does not build as root.
install: ## Install floe into $(BINDIR), /usr/local/bin by default
	@test -f floe || $(MAKE) build
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 755 floe "$(DESTDIR)$(BINDIR)/floe"

uninstall: ## Remove floe from $(BINDIR)
	rm -f "$(DESTDIR)$(BINDIR)/floe"

test: ## Run every test: unit, architecture and end-to-end
	go test ./internal/... ./cmd/...
	$(MAKE) e2e

test-short: ## Run the tests, skipping the slower end-to-end ones
	go test -short ./...

check: fmt-check vet unit e2e ## Run everything CI runs

fmt-check: ## Fail if any file is not gofmt-ed
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

vet: ## Run go vet
	go vet ./...

unit: ## Run the unit tests with the race detector
	go test -race ./internal/...

# The end-to-end tests build floe themselves, so Go cannot tell when a cached
# result is out of date: -count=1 always runs them.
e2e: ## Run the end-to-end tests
	go test -count=1 ./e2e/...

snapshot: ## Build a release into dist/ without publishing it
	goreleaser release --snapshot --clean

clean: ## Remove build output
	rm -rf floe dist

help: ## List the targets
	@grep -E '^[a-z0-9-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{ printf "  %-12s %s\n", $$1, $$2 }'

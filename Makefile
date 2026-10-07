BIN := almagest
# The binary, the agent plugin, and the Obsidian plugin share one version: plugin.json's.
VERSION ?= $(shell sed -n 's/.*"version": "\([^"]*\)".*/\1/p' .claude-plugin/plugin.json | head -1)
LDFLAGS := -ldflags "-X github.com/nathanaday/almagest/internal/cli.Version=$(VERSION)"
ALMAGEST_HOME ?= $(HOME)/.almagest

.PHONY: build install test vet

build:
	go build $(LDFLAGS) -o build/$(BIN) ./cmd/$(BIN)

# install puts the binary where the plugin's wrapper and setup look first.
install:
	mkdir -p $(ALMAGEST_HOME)/bin
	go build $(LDFLAGS) -o $(ALMAGEST_HOME)/bin/$(BIN) ./cmd/$(BIN)

test:
	go test ./...

vet:
	go vet ./...

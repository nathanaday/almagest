BIN := atlas-obsidian
# The binary, the agent plugin, and the Obsidian plugin share one version: plugin.json's.
VERSION ?= $(shell sed -n 's/.*"version": "\([^"]*\)".*/\1/p' .claude-plugin/plugin.json | head -1)
LDFLAGS := -ldflags "-X github.com/nathanaday/atlas-obsidian/internal/cli.Version=$(VERSION)"
ATLAS_HOME ?= $(HOME)/.atlas

.PHONY: build install test vet

build:
	go build $(LDFLAGS) -o build/$(BIN) ./cmd/$(BIN)

# install puts the binary where the plugin's wrapper and setup look first.
install:
	mkdir -p $(ATLAS_HOME)/bin
	go build $(LDFLAGS) -o $(ATLAS_HOME)/bin/$(BIN) ./cmd/$(BIN)

test:
	go test ./...

vet:
	go vet ./...

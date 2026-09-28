BIN := atlas
# The binary, the agent plugin, and the Obsidian plugin share one version: plugin.json's.
VERSION ?= $(shell sed -n 's/.*"version": "\([^"]*\)".*/\1/p' .claude-plugin/plugin.json | head -1)
LDFLAGS := -ldflags "-X github.com/nathanaday/atlas-obsidian/v2-exp/internal/cli.Version=$(VERSION)"
ATLAS_HOME ?= $(HOME)/.atlas
TEMPLATE := internal/vault/template/obsidian

.PHONY: build install test vet obsidian

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

# obsidian builds the Obsidian plugin and copies it into the binary's template, so
# vault init installs the plugin this binary carries.
obsidian:
	cd obsidian && npm install && npm run build
	cp obsidian/dist/main.js obsidian/dist/manifest.json obsidian/dist/styles.css $(TEMPLATE)/

BIN := almagest
# The binary and the agent plugin share one version: plugin.json's.
VERSION ?= $(shell sed -n 's/.*"version": "\([^"]*\)".*/\1/p' .claude-plugin/plugin.json | head -1)
LDFLAGS := -ldflags "-X github.com/nathanaday/almagest/internal/cli.Version=$(VERSION)"
ALMAGEST_HOME ?= $(HOME)/.almagest
# A release builds with this exact toolchain, so every machine makes the same bytes.
TOOLCHAIN := go1.24.2
RELEASE_FLAGS := -trimpath -buildvcs=false -ldflags "-s -w -X github.com/nathanaday/almagest/internal/cli.Version=$(VERSION)"
PLATFORMS := darwin/arm64 darwin/amd64 linux/arm64 linux/amd64
SHA256 := $(shell command -v sha256sum >/dev/null 2>&1 && echo sha256sum || echo "shasum -a 256")

.PHONY: build install test vet release pin

build:
	go build $(LDFLAGS) -o build/$(BIN) ./cmd/$(BIN)

# install puts a build of this checkout where the plugin's launcher runs it: the folder of
# the plugin's version, and the link that the Obsidian plugin and a shell use.
install:
	mkdir -p $(ALMAGEST_HOME)/bin/$(VERSION)
	go build $(LDFLAGS) -o $(ALMAGEST_HOME)/bin/$(VERSION)/$(BIN) ./cmd/$(BIN)
	ln -sfn $(VERSION)/$(BIN) $(ALMAGEST_HOME)/bin/$(BIN)

test:
	go test ./...

vet:
	go vet ./...

# release builds the binary of each platform into build/release, with its checksums.
release:
	rm -rf build/release && mkdir -p build/release
	for p in $(PLATFORMS); do \
		CGO_ENABLED=0 GOTOOLCHAIN=$(TOOLCHAIN) GOOS=$${p%/*} GOARCH=$${p#*/} \
			go build $(RELEASE_FLAGS) -o build/release/$(BIN)-$(VERSION)-$${p%/*}-$${p#*/} ./cmd/$(BIN) || exit 1; \
	done
	cd build/release && $(SHA256) $(BIN)-* > checksums.txt

# pin copies the checksums into the plugin and writes the launcher that holds them.
pin: release
	cp build/release/checksums.txt release/checksums.txt
	go run ./internal/release/pin

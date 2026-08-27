BINARY  := brown
PKG     := ./cmd/brown
GOFLAGS :=

INSTALL_CANDIDATES := $(HOME)/bin $(HOME)/.bin $(HOME)/.local/bin $(HOME)/local/bin /usr/local/bin
INSTALL_DIR := $(firstword $(foreach d,$(INSTALL_CANDIDATES),$(wildcard $(d))))

.PHONY: build run clean test vet fmt lint check install uninstall bandwidth-server

build:
	go build $(GOFLAGS) -o $(BINARY) $(PKG)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint: vet fmt

check: lint test build

install: build
ifeq ($(INSTALL_DIR),)
	$(error No install directory found. Create one of: $(INSTALL_CANDIDATES))
endif
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "installed to $(INSTALL_DIR)/$(BINARY)"

uninstall:
ifeq ($(INSTALL_DIR),)
	$(error No install directory found)
endif
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "removed $(INSTALL_DIR)/$(BINARY)"

bandwidth-server:
	node bandwidth-server/server.js

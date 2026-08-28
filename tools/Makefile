include versions.mk

SHELL := /bin/sh
BIN_DIR := $(CURDIR)/bin
DOWNLOAD_DIR := $(CURDIR)/downloads

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Linux)
BUF_OS := Linux
PROTOC_OS := linux
else ifeq ($(UNAME_S),Darwin)
BUF_OS := Darwin
PROTOC_OS := osx
else
$(error Unsupported operating system: $(UNAME_S))
endif

ifeq ($(UNAME_M),x86_64)
BUF_ARCH := x86_64
PROTOC_ARCH := x86_64
else ifeq ($(UNAME_M),amd64)
BUF_ARCH := x86_64
PROTOC_ARCH := x86_64
else ifeq ($(UNAME_M),aarch64)
BUF_ARCH := aarch64
PROTOC_ARCH := aarch_64
else ifeq ($(UNAME_M),arm64)
BUF_ARCH := arm64
PROTOC_ARCH := aarch_64
else
$(error Unsupported architecture: $(UNAME_M))
endif

BUF := $(BIN_DIR)/buf
PROTOC := $(BIN_DIR)/protoc
PROTOC_INCLUDE := $(BIN_DIR)/include/google/protobuf/descriptor.proto
BUF_URL := https://github.com/bufbuild/buf/releases/download/v$(BUF_VERSION)/buf-$(BUF_OS)-$(BUF_ARCH)
PROTOC_ARCHIVE := protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-$(PROTOC_ARCH).zip
PROTOC_URL := https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/$(PROTOC_ARCHIVE)

.PHONY: all install versions clean

all: install

install: $(BUF) $(PROTOC_INCLUDE)
	@:

$(BIN_DIR) $(DOWNLOAD_DIR):
	@mkdir -p $@

$(BUF): | $(BIN_DIR) $(DOWNLOAD_DIR)
	@printf 'Installing buf %s...\n' '$(BUF_VERSION)'
	@curl --fail --location --silent --show-error '$(BUF_URL)' --output '$(DOWNLOAD_DIR)/buf.tmp'
	@chmod +x '$(DOWNLOAD_DIR)/buf.tmp'
	@mv '$(DOWNLOAD_DIR)/buf.tmp' '$@'

$(PROTOC_INCLUDE): | $(BIN_DIR) $(DOWNLOAD_DIR)
	@printf 'Installing protoc %s...\n' '$(PROTOC_VERSION)'
	@curl --fail --location --silent --show-error '$(PROTOC_URL)' --output '$(DOWNLOAD_DIR)/$(PROTOC_ARCHIVE).tmp'
	@mv '$(DOWNLOAD_DIR)/$(PROTOC_ARCHIVE).tmp' '$(DOWNLOAD_DIR)/$(PROTOC_ARCHIVE)'
	@if command -v unzip >/dev/null 2>&1; then \
		unzip -q -o '$(DOWNLOAD_DIR)/$(PROTOC_ARCHIVE)' -d '$(BIN_DIR)'; \
	elif command -v python3 >/dev/null 2>&1; then \
		python3 -m zipfile -e '$(DOWNLOAD_DIR)/$(PROTOC_ARCHIVE)' '$(BIN_DIR)'; \
	else \
		printf '%s\n' 'error: installing protoc requires unzip or python3' >&2; \
		exit 1; \
	fi
	@mv '$(BIN_DIR)/bin/protoc' '$(PROTOC)'
	@chmod +x '$(PROTOC)'
	@rmdir '$(BIN_DIR)/bin'
	@touch '$@'

versions: install
	@$(BUF) --version
	@$(PROTOC) --version

clean:
	@rm -rf '$(BIN_DIR)' '$(DOWNLOAD_DIR)'

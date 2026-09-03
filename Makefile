TOOLS_DIR := $(CURDIR)/tools
BUF := $(TOOLS_DIR)/bin/buf
PROTOC := $(TOOLS_DIR)/bin/protoc
GO_PACKAGES := ./platform/... ./services/accounts/... ./services/accounts-legacy/... ./services/contacts/... ./services/transactions/... ./services/users/...
GO_FILES := $(shell find platform services -type f -name '*.go')

.PHONY: help tools tools-clean tools-versions proto proto-format proto-lint proto-build proto-check go-format go-format-check go-lint test check smoke-auth

help:
	@printf '%s\n' \
		'make tools          Install pinned project tools under tools/bin' \
		'make tools-versions Show the installed tool versions' \
		'make proto          Format, lint, and generate protobuf contracts' \
		'make proto-check    Check protobuf formatting, lint, and compilation' \
		'make go-format      Format all Go source files' \
		'make go-format-check Check Go source formatting without changing files' \
		'make go-lint        Run go vet for all Go workspace packages' \
		'make test           Run all Go tests in the workspace' \
		'make smoke-auth     Run the Keycloak/RBAC/ownership integration gate' \
		'make check          Run all formatting, linting, contract, and test checks'

tools:
	@$(MAKE) --no-print-directory -C $(TOOLS_DIR) install

tools-clean:
	@$(MAKE) --no-print-directory -C $(TOOLS_DIR) clean

tools-versions: tools
	@$(MAKE) --no-print-directory -C $(TOOLS_DIR) versions

proto-format: tools
	@cd protos && $(BUF) format -w

proto-lint: tools
	@cd protos && $(BUF) lint

proto-build: tools
	@cd protos && $(BUF) build

proto: proto-format proto-lint
	@cd protos && $(BUF) generate
	@cd protos && $(BUF) build

proto-check: tools
	@cd protos && $(BUF) format --diff --exit-code
	@cd protos && $(BUF) lint
	@cd protos && $(BUF) build

go-format:
	@gofmt -w $(GO_FILES)

go-format-check:
	@unformatted="$$(gofmt -l $(GO_FILES))" || exit $$?; \
	if [ -n "$$unformatted" ]; then \
		printf '%s\n' 'The following Go files need formatting:' "$$unformatted"; \
		exit 1; \
	fi

go-lint:
	@go vet $(GO_PACKAGES)

test:
	@go test $(GO_PACKAGES)

check: proto-check go-format-check go-lint test

smoke-auth:
	@./scripts/smoke-auth.sh

TOOLS_DIR := $(CURDIR)/tools
BUF := $(TOOLS_DIR)/bin/buf
PROTOC := $(TOOLS_DIR)/bin/protoc

.PHONY: help tools tools-clean tools-versions proto proto-format proto-lint proto-build proto-check test check smoke-auth

help:
	@printf '%s\n' \
		'make tools          Install pinned project tools under tools/bin' \
		'make tools-versions Show the installed tool versions' \
		'make proto          Format, lint, and generate protobuf contracts' \
		'make proto-check    Check protobuf formatting, lint, and compilation' \
		'make test           Run all Go tests in the workspace' \
		'make smoke-auth     Run the Keycloak/RBAC/ownership integration gate' \
		'make check          Run proto-check and test'

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

test:
	@go test ./platform/... ./services/accounts/... ./services/accounts-legacy/... ./services/contacts/... ./services/transactions/... ./services/users/...

check: proto-check test

smoke-auth:
	@./scripts/smoke-auth.sh

# Environment
PATH := /usr/local/go/bin:$(PATH)
GOROOT := $(shell go env GOROOT)
GOPATH := $(shell go env GOPATH)

# Directories
BACKEND_DIR     := backend
UI_DIR          := frontend
PERF_DIR        := perf
CONTRACTS_DIR   := contracts
BACK_WASM_DIR   := $(BACKEND_DIR)/browser
PERF_K6_DIR     := $(PERF_DIR)/k6

# Artefact dirs
SERVER_ENTRY    := cmd/server/main.go

# Proto output dirs
SVC_PB_OUT      := $(BACKEND_DIR)/pb
PERF_PB_OUT     := $(PERF_K6_DIR)/pb
UI_PB_OUT       := src/lib/pb

# WASM paths
WASM_SRC_DIR    := $(BACKEND_DIR)/cmd/browser
WASM_OUTPUT     := chess.wasm
UI_WASM_DIR     := $(UI_DIR)/static/wasm

# Build All
all: backend frontend perf

protos: proto-backend proto-frontend

# Reusable
define protoc_go
	mkdir -p $(1)
	protoc \
		--go_opt=paths=source_relative \
		--go_out=$(1) \
		--proto_path $(CONTRACTS_DIR) \
		$(CONTRACTS_DIR)/messages.proto
endef

define protoc_ts
	cd $(1) && mkdir -p $(2) && npx protoc \
		--ts_out $(2) \
		--proto_path ../$(CONTRACTS_DIR) \
		../$(CONTRACTS_DIR)/messages.proto
endef

# Backend Build
backend: generate-go proto-backend

generate-go:
	cd $(BACKEND_DIR) && go generate ./...

proto-backend:
	$(call protoc_go,$(SVC_PB_OUT))

# Frontend Build
frontend: install-modules proto-frontend install-wasm

install-modules:
	cd $(UI_DIR) && npm install

proto-frontend:
	$(call protoc_ts,$(UI_DIR),$(UI_PB_OUT))

install-wasm:
	cd $(WASM_SRC_DIR) && GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT) -tags=browser
	mkdir -p $(UI_WASM_DIR)
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" "$(UI_WASM_DIR)/wasm_exec.js"
	cp $(WASM_SRC_DIR)/$(WASM_OUTPUT) $(UI_WASM_DIR)/$(WASM_OUTPUT)

# Perf Build
perf: proto-perf k6-build

proto-perf:
	$(call protoc_go,$(PERF_PB_OUT))

k6-build:
	cd $(PERF_K6_DIR) && go build -o ./k6 .
	cd $(PERF_K6_DIR) && ./k6 version

# Testing
backend-test:
	cd $(BACKEND_DIR) && go test $$(go list ./... | grep -v '^.*/cmd|/wasm/') -timeout=60s

wasm-test:
	cd $(BACK_WASM_DIR) && GOOS=js GOARCH=wasm go test -timeout=60s -exec $(GOPATH)/bin/wasmbrowsertest

perf-test:
	./$(PERF_K6_DIR)/k6 version
# 	./$(PERF_K6_DIR)/k6 run ./$(PERF_DIR)/http/scripts/restapi.ts
	./$(PERF_K6_DIR)/k6 run ./$(PERF_DIR)/http/scripts/gamesockets.ts

test: backend-test wasm-test perf-test

# Prerequisites
install-backend:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	go install go.uber.org/mock/mockgen@v0.6.0

	go install github.com/agnivade/wasmbrowsertest@v0.11.0

install-database:
	go install github.com/pressly/goose/v3/cmd/goose@v3.27.0

clean:
	rm -f $(BACKEND_DIR)/db/sqlc
	rm -f $(BACKEND_DIR)/pb
	rm -f $(WASM_SRC_DIR)/$(WASM_OUTPUT)
	rm -f $(UI_WASM_DIR)/$(WASM_OUTPUT)

.PHONY: all clean
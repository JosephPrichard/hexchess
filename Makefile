# Environment
PATH := /usr/local/go/bin:$(PATH)
GOROOT := $(shell go env GOROOT)
GOPATH := $(shell go env GOPATH)

# Directories
SVC_DIR         := hexchess-svc
UI_DIR          := hexchess-ui
PB_DIR          := hexchess-contracts
SVC_WASM_DIR    := $(SVC_DIR)/browser

# Artefact dirs
SERVER_ENTRY    := cmd/server/main.go

# Proto output dirs
SVC_PB_OUT      := $(SVC_DIR)/pb
UI_PB_OUT       := src/lib/pb

# WASM paths
WASM_SRC_DIR    := $(SVC_DIR)/cmd/browser
WASM_OUTPUT     := chess.wasm
UI_WASM_DIR     := $(UI_DIR)/static/wasm

all: sources

# Backend Build
generate-go:
	cd $(SVC_DIR) && go generate ./...

proto-backend:
	mkdir -p $(SVC_PB_OUT)
	protoc \
		--go_opt=paths=source_relative \
		--go_out=$(SVC_PB_OUT) \
		--proto_path $(PB_DIR) \
		$(PB_DIR)/messages.proto

# Frontend Build
proto-frontend:
	cd $(UI_DIR) && mkdir -p $(UI_PB_OUT) && npx protoc \
		--ts_out $(UI_PB_OUT) \
		--proto_path ../$(PB_DIR) \
		../$(PB_DIR)/messages.proto

install-wasm:
	cd $(WASM_SRC_DIR) && GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT) -tags=browser
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" $(UI_WASM_DIR)
	mkdir -p $(UI_WASM_DIR)
	cp $(WASM_SRC_DIR)/$(WASM_OUTPUT) $(UI_WASM_DIR)/$(WASM_OUTPUT)

# Both build
protos: proto-backend proto-frontend

sources: generate-go protos install-wasm

# Testing
test-server:
	cd $(SVC_DIR) && go test $$(go list ./... | grep -v '^.*/cmd|/wasm/') -timeout=60s

test-wasm:
	cd $(SVC_WASM_DIR) && GOOS=js GOARCH=wasm go test -timeout=60s -exec $(GOPATH)/bin/wasmbrowsertest

test: test-server test-wasm

# Prerequisites
install:
	go install github.com/agnivade/wasmbrowsertest@v0.11.0
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	go install github.com/pressly/goose/v3/cmd/goose@v3.27.0
	go install go.uber.org/mock/mockgen@v0.6.0

clean:
	rm -f $(WASM_SRC_DIR)/$(WASM_OUTPUT)
	rm -f $(UI_WASM_DIR)/$(WASM_OUTPUT)

.PHONY: all clean

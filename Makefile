# Environment
PATH := /usr/local/go/bin:$(PATH)
GOROOT := $(shell go env GOROOT)
GOPATH := $(shell go env GOPATH)

# Directories
SVC_DIR         := hexchess-svc
UI_DIR          := hexchess-ui
PB_DIR          := hexchess-pb
SVC_WASM_DIR    := $(SVC_DIR)/wasm

# Artefact dirs
SERVER_ENTRY    := cmd/server/main.go

# Proto output dirs
SVC_PB_OUT      := $(SVC_DIR)/pb
UI_PB_OUT       := src/lib/pb

# WASM paths
WASM_SRC_DIR    := $(SVC_DIR)/cmd/wasm
WASM_OUTPUT     := chess.wasm
UI_WASM_DIR     := $(UI_DIR)/static/wasm

all: sources ci
sources: generate-go protos install-wasm

# Backend Build
generate-go:
	cd $(SVC_DIR) && go generate ./...

protos: proto-backend proto-frontend

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
	cd $(WASM_SRC_DIR) && GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT) -tags=wasm
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" $(UI_WASM_DIR)
	mkdir -p $(UI_WASM_DIR)
	cp $(WASM_SRC_DIR)/$(WASM_OUTPUT) $(UI_WASM_DIR)/$(WASM_OUTPUT)

ci-server:
	cd $(SVC_DIR) && go test $$(go list ./... | grep -v '^.*/cmd|/wasm/') -timeout=60s

ci-wasm:
	cd $(SVC_WASM_DIR) && GOOS=js GOARCH=wasm go test -timeout=60s -exec $(GOPATH)/bin/wasmbrowsertest

ci: ci-server ci-wasm

install:
	go install github.com/agnivade/wasmbrowsertest@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest

clean:
	rm -f $(WASM_SRC_DIR)/$(WASM_OUTPUT)
	rm -f $(UI_WASM_DIR)/$(WASM_OUTPUT)

.PHONY: all clean

# Environment
PATH := /usr/local/go/bin:$(PATH)
GOROOT := $(shell go env GOROOT)

# Directories
SVC_DIR         := hexchess-svc
UI_DIR          := hexchess-ui
PB_DIR          := hexchess-pb

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
sources: generate-go generate-protos build-wasm install-wasm

generate-go:
	@echo "Generating go sources"
	cd $(SVC_DIR) && go generate ./...

generate-protos: proto-backend proto-frontend

proto-backend:
	@echo "Generating backend proto"
	mkdir -p $(SVC_PB_OUT)
	protoc \
		--go_opt=paths=source_relative \
		--go_out=$(SVC_PB_OUT) \
		--proto_path $(PB_DIR) \
		$(PB_DIR)/messages.proto

proto-frontend:
	@echo "Generating frontend proto"
	cd $(UI_DIR) && mkdir -p $(UI_PB_OUT) && npx protoc \
		--ts_out $(UI_PB_OUT) \
		--proto_path ../$(PB_DIR) \
		../$(PB_DIR)/messages.proto

build-wasm:
	@echo "Compiling WASM chesslib"
	cd $(WASM_SRC_DIR) && GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT) -tags=wasm

install-wasm:
	@echo "Installing WASM into UI"
	@echo "Copying wasm_exec.js from $(GOROOT)"
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" $(UI_WASM_DIR)
	mkdir -p $(UI_WASM_DIR)
	cp $(WASM_SRC_DIR)/$(WASM_OUTPUT) $(UI_WASM_DIR)/$(WASM_OUTPUT)

ci:
	@echo "Running server tests"
	cd $(SVC_DIR) && go test $$(go list ./... | grep -v '^.*/cmd') -timeout=60s

clean:
	@echo "Cleaning WASM output"
	rm -f $(WASM_SRC_DIR)/$(WASM_OUTPUT)
	rm -f $(UI_WASM_DIR)/$(WASM_OUTPUT)

.PHONY: all clean

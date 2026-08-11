# Directories
BACKEND_DIR     := backend
UI_DIR          := frontend
PERF_DIR        := perf
CONTRACTS_DIR   := contracts
BACKEND_CMD_DIR := $(BACKEND_DIR)/cmd
BACK_WASM_DIR   := $(BACKEND_DIR)/browser
BACK_API_DIR    := $(BACKEND_CMD_DIR)/server/api
BACK_CONS_DIR   := $(BACKEND_CMD_DIR)/server/consumers
BACK_JOB_DIR    := $(BACKEND_CMD_DIR)/server/jobs
PERF_QUE_DIR    := $(BACKEND_CMD_DIR)/scripts/perftest
PERF_K6_DIR     := $(PERF_DIR)/k6
PERF_HTTP_DIR   := $(PERF_DIR)/http/scripts

# Proto output dirs
SVC_PB_OUT      := $(BACKEND_DIR)/pb
PERF_PB_K6_OUT  := $(PERF_K6_DIR)/pb
UI_PB_OUT       := src/lib/pb

# WASM paths
WASM_SRC_DIR    := $(BACKEND_DIR)/cmd/browser
WASM_OUTPUT     := chess.wasm
UI_WASM_DIR     := $(UI_DIR)/static/wasm

# Dependency paths
VTPROTO := $(shell cd $(BACKEND_DIR) && go list -m -f '{{.Dir}}' github.com/planetscale/vtprotobuf)

# Build

.PHONY: all backend frontend chess-wasm k6 test perf-test clean

all: backend frontend k6

backend:
	# SQLc and mockgen
	cd $(BACKEND_DIR) && go generate ./...
	# Protoc codegen
	mkdir -p $(SVC_PB_OUT)
	protoc \
		-I $(VTPROTO)/include \
		--go_out=$(SVC_PB_OUT) \
		--go_opt=paths=source_relative \
		--proto_path $(CONTRACTS_DIR) \
		--go-vtproto_out=$(SVC_PB_OUT) \
		--go-vtproto_opt=paths=source_relative \
		--go-vtproto_opt=features=marshal+unmarshal+size \
		$(CONTRACTS_DIR)/messages.proto \

frontend:
    # Protoc codegen
	( \
		cd $(UI_DIR); \
		mkdir -p $(UI_PB_OUT); \
		npx protoc \
			--ts_out $(UI_PB_OUT) \
			--proto_path ../$(CONTRACTS_DIR) \
			../$(CONTRACTS_DIR)/messages.proto \
    )

chess-wasm:
	# WASM compilation
	cd $(WASM_SRC_DIR) && GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT) -tags=browser
	mkdir -p $(UI_WASM_DIR)
	cp $(WASM_SRC_DIR)/$(WASM_OUTPUT) $(UI_WASM_DIR)

k6:
	# Protoc codegen
	mkdir -p $(PERF_PB_K6_OUT)
	protoc \
		--go_opt=paths=source_relative \
		--go_out=$(PERF_PB_K6_OUT) \
		--proto_path $(CONTRACTS_DIR) \
		$(CONTRACTS_DIR)/messages.proto
	# Compile k6 binary
	cd $(PERF_K6_DIR) && go build -o ./k6 .
	cd $(PERF_K6_DIR) && ./k6 version

# Vulnerability check
vulncheck:
	cd backend && govulncheck ./...

# Testing
functional-test:
	# Backend server test (-p 1 is required to force tests to run serially)
	cd $(BACKEND_DIR) && go test $$(go list ./... | grep -v '^.*/cmd|/wasm/') -p 1 -timeout=30s

wasm-test:
	# Backend wasm module test
	cd $(BACK_WASM_DIR) && GOOS=js GOARCH=wasm go test -coverprofile=coverage_browser.out -v -tags=browser -timeout=10s -exec wasmbrowsertest

perf-test:
	# k6 HTTP perf tests
	./$(PERF_K6_DIR)/k6 run ./$(PERF_HTTP_DIR)/restapi.ts
	./$(PERF_K6_DIR)/k6 run ./$(PERF_HTTP_DIR)/gamesockets.ts
	# Job queue perf tests
	go run ./$(PERF_QUE_DIR)/main.go

clean:
	rm -rf $(BACKEND_DIR)/db/metricsdb
	rm -rf $(BACKEND_DIR)/db/sqlc
	rm -rf $(SVC_PB_OUT)
	find ./$(BACKEND_DIR) -type f -name "*_mock.go" -delete
	find ./$(BACKEND_DIR) -type f -name "*_wasm" -delete

	rm -rf $(UI_DIR)/build
	rm -rf $(UI_DIR)$(UI_PB_OUT)

	rm -rf $(PERF_PB_K6_OUT)
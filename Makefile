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

.PHONY:
	all
	backend
	backend-generate
	backend-pb
	frontend
	frontend-wasm
	frontend-js
	k6
	test
	perf-test
	clean

all: backend frontend k6

backend: backend-generate backend-pb

backend-generate:
	# SQLc and mockgen
	cd $(BACKEND_DIR) && go generate ./...

backend-pb:
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

frontend: frontend-wasm frontend-js

frontend-wasm: backend-pb
	# WASM compilation
	mkdir -p $(UI_WASM_DIR)/$(WASM_OUTPUT)
	cd $(WASM_SRC_DIR) && GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT) -tags=browser
	cp $(WASM_SRC_DIR)/$(WASM_OUTPUT) $(UI_WASM_DIR)/$(WASM_OUTPUT)

frontend-js:
    # Protoc codegen
	( \
		cd $(UI_DIR); \
		mkdir -p $(UI_PB_OUT); \
		npx protoc \
			--ts_out $(UI_PB_OUT) \
			--proto_path ../$(CONTRACTS_DIR) \
			../$(CONTRACTS_DIR)/messages.proto \
    )

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

# Testing
test:
	# Backend server test
	cd $(BACKEND_DIR) && go test $$(go list ./... | grep -v '^.*/cmd|/wasm/') -timeout=60s
	# Backend wasm module test
	cd $(BACK_WASM_DIR) && GOOS=js GOARCH=wasm go test -tags=browser -timeout=60s -exec wasmbrowsertest

perf-test:
	# k6 HTTP perf tests
	./$(PERF_K6_DIR)/k6 run ./$(PERF_HTTP_DIR)/restapi.ts
	./$(PERF_K6_DIR)/k6 run ./$(PERF_HTTP_DIR)/gamesockets.ts
	# Job queue perf tests
	go run ./$(PERF_QUE_DIR)/main.go

clean:
	rm -rf $(BACKEND_DIR)/db/metricsdb
	rm -rf $(BACKEND_DIR)/db/primarydb
	rm -rf $(SVC_PB_OUT)
	rm $(BACKEND_DIR)/cloud/google_mock.go
	rm $(BACKEND_DIR)/cmd/browser/chess.wasm

	rm -rf $(UI_DIR)/build
	rm -rf $(UI_DIR)$(UI_PB_OUT)
	rm $(UI_DIR)/static/wasm/chess.wasm

	rm -rf $(PERF_PB_K6_OUT)
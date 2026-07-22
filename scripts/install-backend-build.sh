#!/usr/bin/env bash
set -euo pipefail

PROTOC_GEN_GO_VERSION="v1.36.11"
VTPROTO_VERSION="v0.6.0"
SQLC_VERSION="v1.31.1"
MOCKGEN_VERSION="v0.6.0"

go install -v "google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}"
go install -v "github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@${VTPROTO_VERSION}"
go install -v "github.com/sqlc-dev/sqlc/cmd/sqlc@${SQLC_VERSION}"
go install -v "go.uber.org/mock/mockgen@${MOCKGEN_VERSION}"
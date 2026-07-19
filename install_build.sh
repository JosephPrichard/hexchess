#!/usr/bin/env bash
set -euo pipefail

PROTOC_GEN_GO_VERSION="latest"
VTPROTO_VERSION="latest"
SQLC_VERSION="latest"
MOCKGEN_VERSION="latest"
WASMBROWSERTEST_VERSION="latest"
GOOSE_VERSION="latest"

go install "google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}"
go install "github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@${VTPROTO_VERSION}"
go install "github.com/sqlc-dev/sqlc/cmd/sqlc@${SQLC_VERSION}"
go install "go.uber.org/mock/mockgen@${MOCKGEN_VERSION}"
go install "github.com/agnivade/wasmbrowsertest@${WASMBROWSERTEST_VERSION}"
go install "github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}"
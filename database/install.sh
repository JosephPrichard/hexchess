#!/usr/bin/env bash
set -euo pipefail

GOOSE_VERSION="latest"

go install "github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}"
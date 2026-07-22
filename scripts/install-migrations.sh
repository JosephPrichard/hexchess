#!/usr/bin/env bash
set -euo pipefail

GOOSE_VERSION="v3.27.2"
RIVER_VERSION="v0.40.0"

go install "github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}"
go install "github.com/riverqueue/river/cmd/river@${RIVER_VERSION}"
#!/usr/bin/env bash
set -euo pipefail

VULNCHECK_VERSION="v1.6.0"

go install "golang.org/x/vuln/cmd/govulncheck@${VULNCHECK_VERSION}"
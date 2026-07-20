#!/usr/bin/env bash
set -euo pipefail

WASMBROWSERTEST_VERSION="latest"

go install "github.com/agnivade/wasmbrowsertest@${WASMBROWSERTEST_VERSION}"
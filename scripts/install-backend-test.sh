#!/usr/bin/env bash
set -euo pipefail

WASMBROWSERTEST_VERSION="0.11.0"

go install "github.com/agnivade/wasmbrowsertest@${WASMBROWSERTEST_VERSION}"
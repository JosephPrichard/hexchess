#!/usr/bin/env fish

set VULNCHECK_VERSION "v1.6.0"

go install "golang.org/x/vuln/cmd/govulncheck@$VULNCHECK_VERSION"
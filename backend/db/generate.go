package db

import _ "embed"

//go:generate sqlc generate

//go:embed schema-primary.sql
var CreatePrimarySchema string

//go:embed schema-metrics.sql
var CreateMetricsSchema string

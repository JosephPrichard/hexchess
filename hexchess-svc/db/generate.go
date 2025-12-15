package db

import _ "embed"

//go:generate sqlc generate

//go:embed schema.sql
var CreateSchema string

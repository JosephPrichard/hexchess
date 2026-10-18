package database

import _ "embed"

//go:generate sqlc generate

//go:embed sql/schema.sql
var CreatePrimarySchema string

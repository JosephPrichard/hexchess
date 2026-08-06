package database

import _ "embed"

//go:generate sqlc generate

//go:embed schema.sql
var CreatePrimarySchema string

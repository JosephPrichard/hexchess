package static

import (
	"embed"
	_ "embed"
)

//go:embed all:test
var Mocks embed.FS

//go:embed countries.json
var CountryListJson []byte

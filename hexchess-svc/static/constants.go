package static

import (
	_ "embed"
)

//go:embed countries.json
var CountryListFile []byte

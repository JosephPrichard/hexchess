package meta

import (
	_ "embed"
)

//go:embed countries.json
var CountryListFile []byte

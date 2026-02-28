package assets

import (
	"embed"
	
)

//go:embed all:test
var Mocks embed.FS

//go:embed countries.json
var CountryListJson []byte

//go:embed default-profile-pic.jpg
var DefaultProfilePic []byte

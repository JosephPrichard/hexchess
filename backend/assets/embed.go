package assets

import (
	_ "embed"
	"encoding/json"
)

//go:embed countries.json
var CountryListJson []byte

//go:embed default-profile-pic.jpg
var DefaultProfilePic []byte

func GetCountryList() []string {
	var countryList []string
	err := json.Unmarshal(CountryListJson, &countryList)
	if err != nil {
		panic(err)
	}
	return countryList
}

package network

import (
	"encoding/json"
	"hexchess-svc/assets"
	"hexchess-svc/utils/slogutil"
)

type StaticData struct {
	validCountries map[string]bool
	countryList    []string
}

func NewStaticData() StaticData {
	var countryList []string
	if err := json.Unmarshal(assets.CountryListJson, &countryList); err != nil {
		slogutil.Fatal("unmarshal country list", err)
	}
	if countryList == nil {
		countryList = []string{}
	}
	validCountries := make(map[string]bool)
	for _, country := range countryList {
		validCountries[country] = true
	}
	return StaticData{validCountries: validCountries, countryList: countryList}
}

package util

import (
	"encoding/json"
	"strings"
)

func AsJSONReader(v any) *strings.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic("json.Marshal failed: " + err.Error())
	}
	return strings.NewReader(string(b))
}

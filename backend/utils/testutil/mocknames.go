package testutil

import (
	"hexchess-svc/utils/slogutil"
	"reflect"

	"uuid"
)

func NewTestNames[T any](d T) *T {
	names := d
	namesPtr := &names

	reflectNames := reflect.ValueOf(namesPtr)
	if reflectNames.Kind() == reflect.Pointer {
		reflectNames = reflectNames.Elem()
	}
	if reflectNames.Kind() != reflect.Struct {
		slogutil.Fatal("reflectNames is not a struct", nil)
	}

	for _, field := range reflectNames.Fields() {
		if field.Kind() == reflect.String && field.CanSet() {
			newValue := field.String() + "-" + uuid.NewV7().String()
			field.SetString(newValue)
		}
	}
	return namesPtr
}

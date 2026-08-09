package testutil

import (
	"hexchess-svc/utils/alog"
	"reflect"

	"github.com/google/uuid"
)

func NewTestNames[T any](d T) *T {
	names := d
	namesPtr := &names

	reflectNames := reflect.ValueOf(namesPtr)
	if reflectNames.Kind() == reflect.Pointer {
		reflectNames = reflectNames.Elem()
	}
	if reflectNames.Kind() != reflect.Struct {
		alog.Fatal("reflectNames is not a struct", nil)
	}

	for _, field := range reflectNames.Fields() {
		if field.Kind() == reflect.String && field.CanSet() {
			newValue := field.String() + "-" + uuid.New().String()
			field.SetString(newValue)
		}
	}
	return namesPtr
}

package testutil

import (
	"github.com/google/uuid"
	"hexchess-svc/lib/logutil"
	"reflect"
)

func MakeTestNames[T any](d T) *T {
	names := d
	namesPtr := &names

	reflectNames := reflect.ValueOf(namesPtr)
	if reflectNames.Kind() == reflect.Ptr {
		reflectNames = reflectNames.Elem()
	}
	if reflectNames.Kind() != reflect.Struct {
		logutil.Fatal("reflectNames is not a struct")
	}

	for i := range reflectNames.NumField() {
		field := reflectNames.Field(i)

		if field.Kind() == reflect.String && field.CanSet() {
			newValue := field.String() + "-" + uuid.New().String()
			field.SetString(newValue)
		}
	}
	return namesPtr
}

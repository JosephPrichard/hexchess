package testutil

import (
	"encoding/json/v2"
	"io"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func Equal[T any](t *testing.T, expected, actual T, opts ...cmp.Option) bool {
	t.Helper()
	diff := cmp.Diff(expected, actual, opts...)
	isEqual := diff == ""
	if !isEqual {
		t.Errorf("\n%s", diff)
	}
	return isEqual
}

func AssertRespBody[V any](t *testing.T, wantBody V, w *httptest.ResponseRecorder, opts ...cmp.Option) {
	t.Helper()
	diff, bodyString := assertRespBody(wantBody, w, opts...)
	t.Logf("response body:\n%s", bodyString)
	if diff != "" {
		t.Error(diff)
	}
}

func assertRespBody[V any](wantBody V, w *httptest.ResponseRecorder, opts ...cmp.Option) (string, string) {
	resp := w.Result()
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err.Error(), ""
	}
	str := string(bytes)

	var actualBody V
	if err = json.Unmarshal(bytes, &actualBody); err != nil {
		return err.Error(), ""
	}
	return cmp.Diff(wantBody, actualBody, opts...), str
}

func CmpIgnoreExcept(target any, keepFields ...string) cmp.Option {
	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var toIgnore []string
	for i := range t.NumField() {
		name := t.Field(i).Name
		if !slices.Contains(keepFields, name) {
			toIgnore = append(toIgnore, name)
		}
	}

	return cmpopts.IgnoreFields(target, toIgnore...)
}

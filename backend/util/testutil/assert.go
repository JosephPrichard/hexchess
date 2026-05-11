package testutil

import (
	"encoding/json"
	"github.com/google/go-cmp/cmp/cmpopts"
	"io"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err.Error(), ""
	}
	var actualBody V
	if err = json.Unmarshal(b, &actualBody); err != nil {
		return err.Error(), ""
	}
	return cmp.Diff(wantBody, actualBody, opts...), string(b)
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

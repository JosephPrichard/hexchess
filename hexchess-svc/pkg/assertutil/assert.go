package assertutil

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"io"
	"net/http/httptest"
	"testing"
)

func Equal[T any](t *testing.T, expected, actual T, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(expected, actual, opts...); diff != "" {
		t.Errorf("\n%s", diff)
	}
}

func AssertRespBody[V any](t *testing.T, wantBody any, w *httptest.ResponseRecorder, opts ...cmp.Option) {
	t.Helper()
	str := assertRespBody[V](wantBody, w, opts...)
	if str != "" {
		t.Error(str)
	}
}

func assertRespBody[V any](wantBody any, w *httptest.ResponseRecorder, opts ...cmp.Option) string {
	resp := w.Result()
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err.Error()
	}

	switch wantBody := wantBody.(type) {
	case V:
		var actualBody V
		if err = json.Unmarshal(b, &actualBody); err != nil {
			return err.Error()
		}
		return cmp.Diff(wantBody, actualBody, opts...)
	default:
		return fmt.Sprintf("unsupported type in body assert: %T", wantBody)
	}
}

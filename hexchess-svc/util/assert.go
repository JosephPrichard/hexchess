package util

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http/httptest"
	"testing"
)

func AssertEqualIgnoring[T any](t *testing.T, expected, actual T, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(expected, actual, opts...); diff != "" {
		t.Errorf("\n%s", diff)
	}
}
func AssertRespBody[V any](t *testing.T, expBody any, w *httptest.ResponseRecorder, opts ...cmp.Option) {
	resp := w.Result()
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	switch expBody := expBody.(type) {
	case V:
		var actualBody V
		if err = json.Unmarshal(b, &actualBody); err != nil {
			t.Fatal(err)
		}
		AssertEqualIgnoring[V](t, expBody, actualBody, opts...)
	default:
		assert.Fail(t, fmt.Sprintf("unsupported type in body assert: %T", expBody))
	}
}

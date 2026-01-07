package assertutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http/httptest"
	"testing"
)

// ElementsMatch is an algorithm taken from testify/assert that is modified to use go-cmp
func ElementsMatch[T any](t *testing.T, listA, listB []T, opts ...cmp.Option) {
	t.Helper()
	aLen := len(listA)
	bLen := len(listB)

	var extraA, extraB []T

	visited := make([]bool, bLen)
	for i := range aLen {
		element := listA[i]
		found := false
		for j := range bLen {
			if visited[j] {
				continue
			}
			if cmp.Equal(listB[j], element, opts...) {
				visited[j] = true
				found = true
				break
			}
		}
		if !found {
			extraA = append(extraA, element)
		}
	}

	for j := range bLen {
		if visited[j] {
			continue
		}
		extraB = append(extraB, listB[j])
	}

	if len(extraA) == 0 && len(extraB) == 0 {
		return
	}

	var sb bytes.Buffer

	sb.WriteString("elements differ")
	if len(extraA) > 0 {
		sb.WriteString("\n\nextra elements in list A:\n")
		for _, e := range extraA {
			sb.WriteString(fmt.Sprintf("{%+v},\n", e))
		}
	}
	if len(extraB) > 0 {
		sb.WriteString("\nextra elements in list B:\n")
		for _, e := range extraB {
			sb.WriteString(fmt.Sprintf("{%+v},\n", e))
		}
	}
	sb.WriteString("\nlistA:\n")
	sb.WriteString(fmt.Sprintf("%+v\n", listA))
	sb.WriteString("\nlistB:\n")
	sb.WriteString(fmt.Sprintf("%+v\n\n", listB))

	t.Error(sb.String())
}

func Equal[T any](t *testing.T, expected, actual T, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(expected, actual, opts...); diff != "" {
		t.Errorf("\n%s", diff)
	}
}

func AssertRespBody[V any](t *testing.T, wantBody any, w *httptest.ResponseRecorder, opts ...cmp.Option) {
	t.Helper()
	resp := w.Result()
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	switch wantBody := wantBody.(type) {
	case V:
		var actualBody V
		if err = json.Unmarshal(b, &actualBody); err != nil {
			t.Fatal(err)
		}
		Equal[V](t, wantBody, actualBody, opts...)
	default:
		assert.Fail(t, fmt.Sprintf("unsupported type in body assert: %T", wantBody))
	}
}

package testutil

import (
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
)

type value struct {
	A int `json:"a"`
	B int `json:"b"`
}

type test struct {
	name    string
	body    value
	w       *httptest.ResponseRecorder
	opts    cmp.Option
	wantStr string
}

func testAssertRestBody[V any](t *testing.T, test test) {
	str := assertRespBody(test.body, test.w, test.opts)
	t.Logf("assert rest body:\n%s", str)
	assert.Equal(t, test.wantStr, str)
}

func TestAssertRespBody(t *testing.T) {
	
	testAssertRestBody[value](t, test{
		name: "equal",
		body: value{A: 1, B: 3},
		w: func() *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			w.Write([]byte(`{"a":1,"b":2}`))
			return w
		}(),
		opts:    cmpopts.IgnoreFields(value{}, "B"),
		wantStr: "",
	})
	testAssertRestBody[value](t, test{
		name: "not equals",
		body: value{A: 1, B: 2},
		w: func() *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			w.Write([]byte(`{"a":2,"b":2}`))
			return w
		}(),
		opts:    cmpopts.IgnoreFields(value{}, "B"),
		wantStr: "  assertutil.value{\n- \tA: 1,\n+ \tA: 2,\n  \t... // 1 ignored field\n  }\n",
	})
	testAssertRestBody[int](t, test{
		name: "wrong type",
		body: value{A: 1, B: 2},
		w: func() *httptest.ResponseRecorder {
			w := httptest.NewRecorder()
			w.Write([]byte(`{"a":1,"b":2}`))
			return w
		}(),
		opts:    cmpopts.IgnoreFields(value{}, "B"),
		wantStr: "unsupported type in body assert: assertutil.value",
	})
}

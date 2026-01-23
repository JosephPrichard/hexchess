package assertutil

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"net/http/httptest"
	"testing"
)

func TestAssertElementsEqual(t *testing.T) {
	type value struct {
		A int
		B int
	}
	for _, test := range []struct {
		name    string
		a       []value
		b       []value
		opts    cmp.Option
		wantStr string
	}{
		{
			name:    "equal",
			a:       []value{{A: 1, B: 1}, {A: 2}, {A: 3, B: 1}},
			b:       []value{{A: 1}, {A: 2, B: 2}, {A: 3}},
			opts:    cmpopts.IgnoreFields(value{}, "B"),
			wantStr: "",
		},
		{
			name:    "not equals extra elements",
			a:       []value{{A: 1, B: 1}, {A: 2}, {A: 3, B: 1}},
			b:       []value{{A: 1}, {A: 2, B: 2}, {A: 3}, {}},
			opts:    cmpopts.IgnoreFields(value{}, "B"),
			wantStr: "elements differ\nextra elements in list B:\n{{A:0 B:0}}\n\nlistA:\n[{A:1 B:1} {A:2 B:0} {A:3 B:1}]\n\nlistB:\n[{A:1 B:0} {A:2 B:2} {A:3 B:0} {A:0 B:0}]\n\n",
		},
		{
			name:    "not equals diff elements",
			a:       []value{{A: 1, B: 1}, {A: 1}, {A: 3, B: 1}},
			b:       []value{{A: 1}, {A: 2, B: 2}, {A: 3}},
			opts:    cmpopts.IgnoreFields(value{}, "B"),
			wantStr: "elements differ\n\nextra elements in list A:\n{{A:1 B:0}}\n\nextra elements in list B:\n{{A:2 B:2}}\n\nlistA:\n[{A:1 B:1} {A:1 B:0} {A:3 B:1}]\n\nlistB:\n[{A:1 B:0} {A:2 B:2} {A:3 B:0}]\n\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			str := elementsMatch(test.a, test.b, test.opts)
			t.Logf("elements match output:\n%s", str)
			assert.Equal(t, test.wantStr, str)
		})
	}
}

type test struct {
	name    string
	body    any
	w       *httptest.ResponseRecorder
	opts    cmp.Option
	wantStr string
}

func testAssertRestBody[V any](t *testing.T, test test) {
	str := assertRespBody[V](test.body, test.w, test.opts)
	t.Logf("assert rest body:\n%s", str)
	assert.Equal(t, test.wantStr, str)
}

func TestAssertRespBody(t *testing.T) {
	type value struct {
		A int `json:"a"`
		B int `json:"b"`
	}
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

package web

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

type ServiceView struct {
	Status  int
	Message string
}

var SuccessJSON = []byte(`{status: 200, message: "SUCCESS",}`)
var EmptyRefreshJSON = []byte(`{message: null}`)

func init() {
	var err error
	if SuccessJSON, err = json.Marshal(ServiceView{Status: 200, Message: "SUCCESS"}); err != nil {
		log.Fatalf("failed to marshal service view: %v", err)
	}
	if EmptyRefreshJSON, err = json.Marshal(RefreshResp{Session: nil}); err != nil {
		log.Fatalf("failed to marshal refresh resp: %v", err)
	}
}

func writeSuccessJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(SuccessJSON)
}

func writeJSON[V any](w http.ResponseWriter, status int, data V) {
	v, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(v)
}

func writeEmptyRefreshJSON(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(EmptyRefreshJSON)
}

func assertRespNoup[V any](t *testing.T, expBody any, w *httptest.ResponseRecorder) {
	assertResp(t, expBody, w, func(*V) {})
}

func assertResp[V any](t *testing.T, expBody any, w *httptest.ResponseRecorder, updtResp func(*V)) {
	resp := w.Result()
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("body: %s", b)

	switch expBody.(type) {
	case string:
		assert.Equal(t, expBody, string(b))
	case V:
		var actualBody V
		if err = json.Unmarshal(b, &actualBody); err != nil {
			t.Fatal(err)
		}
		updtResp(&actualBody)
		assert.Equal(t, expBody, actualBody)
	default:
		assert.Fail(t, fmt.Sprintf("unsupported type: %T", expBody))
	}
}

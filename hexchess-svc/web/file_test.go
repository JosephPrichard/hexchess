package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/egress"
	"hexchess-svc/itest"
	svc "hexchess-svc/services"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleUploadProfilePic(t *testing.T) {
	t.Parallel()

	// given
	services := svc.SetupServicesTest(t, itest.Redis, itest.Aws)
	defer services.Close()

	createTestSessions(t, services)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("testfiledata"))
	writer.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/users/profile-pics", body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Cookie", FmtCookie(TestSessionID2))
	w := httptest.NewRecorder()

	// when
	hander := MakeServeMux(Setup{Services: services})
	hander.ServeHTTP(w, r)

	// then
	assert.Equal(t, http.StatusOK, w.Code)

	var view ServiceView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &view))

	assert.Equal(t, "testfiledata", egress.GetS3Object(t, services.S3Client, services.S3ProfileBucket, view.Message)) // key is contained in the mesage.
}

func TestHandleGetProfilePic(t *testing.T) {
	t.Parallel()

	key1 := fmt.Sprintf("users/profile-pics/3/%s", uuid.NewString())
	key2 := fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString())

	for _, test := range []struct {
		name            string
		userID          string
		wantStatus      int
		wantWithKey     string
		wantWithoutKeys []string
	}{
		{
			name:            "user has profile pic in storage",
			userID:          "1",
			wantStatus:      http.StatusTemporaryRedirect,
			wantWithKey:     key2,
			wantWithoutKeys: []string{key1},
		},
		{
			name:       "user has redirected profile pic",
			userID:     "2",
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(test.userID, func(t *testing.T) { // given
			services := svc.SetupServicesTest(t, itest.Redis, itest.Aws)
			defer services.Close()

			egress.PutS3Object(t, services.S3Client, services.S3ProfileBucket, key1, []byte("testfiledata1"))
			egress.PutS3Object(t, services.S3Client, services.S3ProfileBucket, key2, []byte("testfiledata2"))

			// when
			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/users/profile-pics?userId=%s", test.userID), nil)
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

			// then
			resp := w.Body.String()
			assert.Equal(t, test.wantStatus, w.Code)

			if w.Code == http.StatusTemporaryRedirect {
				t.Logf("got profile pic redirect: %s", resp)
				for _, key := range test.wantWithoutKeys {
					if strings.Contains(resp, key) {
						t.Errorf("expected profile pic redirect to not contain key %s", key)
					}
				}
				if !strings.Contains(resp, test.wantWithKey) {
					t.Fatalf("expected profile pic redirect to contain key %s", test.wantWithKey)
				}
			}
		})
	}
}

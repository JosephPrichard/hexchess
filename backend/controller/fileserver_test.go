package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/service"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHandleUploadProfilePic(t *testing.T) {
	t.Parallel()

	bodyJustRight := "testfiledata"
	bodyTooLarge := bytes.Repeat([]byte{'a'}, (5<<20)+1)

	computeChecksum := func(data []byte) string {
		start := time.Now()

		sha := sha256.New()
		sha.Write(data)
		bodyChecksum := base64.StdEncoding.EncodeToString(sha.Sum(nil))

		t.Logf("computed checksum in %v\n", time.Since(start))
		return bodyChecksum
	}

	tests := []struct {
		name        string
		body        []byte
		contentType string
		checksum    string
		wantStatus  int
		assertS3    func(*testing.T, cloud.AWSClient)
	}{
		{
			name:        "UploadSuccessful",
			body:        []byte(bodyJustRight),
			contentType: "application/octet-stream",
			checksum:    computeChecksum([]byte(bodyJustRight)),
			wantStatus:  http.StatusOK,
			assertS3: func(t *testing.T, client cloud.AWSClient) {
				output, err := client.S3Client.GetObject(t.Context(), &s3.GetObjectInput{
					Bucket: aws.String(client.S3ProfileBucket),
					Key:    aws.String("users/profile-pics/2/00000000-0000-0000-0000-000000000000"),
				})
				if err != nil {
					t.Fatalf("failed to get from s3: %v", err)
				}
				body, err := io.ReadAll(output.Body)
				if err != nil {
					t.Fatalf("failed to drain body from s3: %v", err)
				}
				assert.Equal(t, bodyJustRight, string(body))
			},
		},
		{
			name:       "UploadExceedsLimit",
			body:       bodyTooLarge,
			checksum:   computeChecksum(bodyTooLarge),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, &serviceMocks{Entropy: &svc.StableEntropySource{}}, itest.Redis, itest.AWS)
			defer testinfra.Close()

			cloud.SetupS3Test(t, testinfra.AWS, nil)

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/users/profile-pics", bytes.NewBuffer(tt.body))
			r.Header.Set("Content-Type", tt.contentType)
			r.Header.Set("Cookie", FmtCookie(TestSessionID2))
			r.Header.Set("Content-Digest", tt.checksum)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.assertS3 != nil {
				tt.assertS3(t, testinfra.AWS)
			}
		})
	}
}

func TestHandleGetProfilePic(t *testing.T) {
	t.Parallel()

	profileKey1 := fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString())

	tests := []struct {
		name          string
		userID        string
		wantStatus    int
		wantWithKey   string
		setupTestData func(*testing.T, itest.TestInfra)
	}{
		{
			name:        "HasStoredProfilePic",
			userID:      "1",
			wantStatus:  http.StatusTemporaryRedirect,
			wantWithKey: profileKey1,
			setupTestData: func(t *testing.T, testinfra itest.TestInfra) {
				cloud.SetupS3Test(t, testinfra.AWS, []*s3.PutObjectInput{
					{
						Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
						Key:    aws.String(profileKey1),
						Body:   bytes.NewReader([]byte("test1")),
					},
				})
			},
		},
		{
			name:       "HasDefaultPic",
			userID:     "2",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, testinfra := setupTestHandler(t, nil, itest.Redis, itest.AWS)
			defer testinfra.Close()

			if tt.setupTestData != nil {
				tt.setupTestData(t, testinfra)
			}

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/users/profile-pics?userId=%s", tt.userID), nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			resp := w.Body.String()
			assert.Equal(t, tt.wantStatus, w.Code)

			assert.True(t, len(resp) > 0)

			if w.Code == http.StatusTemporaryRedirect {
				t.Logf("got profile pic redirect: %s", resp)
				if !strings.Contains(resp, tt.wantWithKey) {
					t.Fatalf("expected profile pic redirect to contain key %s", tt.wantWithKey)
				}
			}
		})
	}
}

package controller

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"hexchess-svc/egress"
	"hexchess-svc/itest"
	"hexchess-svc/lib/testutil"
	"hexchess-svc/service"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var s3InputCmpOpts = testutil.CmpIgnoreExcept(s3.PutObjectInput{}, "Bucket", "Key", "ContentType", "CacheControl", "ChecksumSHA256")

type drainingS3Uploader struct {
	*egress.CompositeS3API
}

func (u *drainingS3Uploader) PutObject(ctx context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	doneDraining := make(chan error, 1)

	go func() {
		_, err := io.ReadAll(params.Body)
		doneDraining <- err
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-doneDraining:
		return &s3.PutObjectOutput{}, err
	}
}

func TestHandleUploadProfilePic(t *testing.T) {
	t.Parallel()

	staticBodyData := "testfiledata"

	tests := []struct {
		name        string
		body        []byte
		contentType string
		checksum    string
		wantStatus  int
		setupMocks  func(*testing.T, *gomock.Controller) egress.S3ClientAPI
	}{
		{
			name:        "UploadSuccessful",
			body:        []byte(staticBodyData),
			contentType: "application/octet-stream",
			checksum:    "checksum",
			wantStatus:  http.StatusOK,
			setupMocks: func(t *testing.T, ctrl *gomock.Controller) egress.S3ClientAPI {
				mockS3Client := egress.NewMockS3ClientAPI(ctrl)

				// assert that keys and metadata arrive on the system correctly
				wantPutInput := &s3.PutObjectInput{
					Bucket:         aws.String(egress.S3ProfileBucket),
					Key:            aws.String("users/profile-pics/2/mock-1"),
					ContentType:    aws.String("application/octet-stream"),
					CacheControl:   aws.String("public, max-age=31536000"),
					ChecksumSHA256: aws.String("checksum"),
				}
				mockS3Client.EXPECT().
					PutObject(
						gomock.Any(),
						gomock.Cond(func(input *s3.PutObjectInput) bool {
							recvBodyBytes, _ := io.ReadAll(input.Body)
							return testutil.Equal(t, wantPutInput, input, s3InputCmpOpts) &&
								testutil.Equal(t, staticBodyData, string(recvBodyBytes))
						}),
						gomock.Any()).
					Return(&s3.PutObjectOutput{}, nil)

				// empty keylist, expect to not delete anything
				mockS3Client.EXPECT().
					ListObjectsV2(gomock.Any(), &s3.ListObjectsV2Input{
						Bucket: aws.String(egress.S3ProfileBucket),
						Prefix: aws.String("users/profile-pics/2"),
					}).
					Return(&s3.ListObjectsV2Output{}, nil)

				return mockS3Client
			},
		},
		{
			name:       "UploadExceedsLimit",
			body:       bytes.Repeat([]byte{'a'}, (5<<20)+1),
			wantStatus: http.StatusBadRequest,
			setupMocks: func(t *testing.T, ctrl *gomock.Controller) egress.S3ClientAPI {
				return &drainingS3Uploader{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockS3Client := tt.setupMocks(t, ctrl)

			h, testinfra := setupTestHandler(t, serviceMocks{S3Client: mockS3Client, Entropy: &svc.StableEntropySource{}}, itest.Redis)
			defer testinfra.Close()

			createTestSessions(t, testinfra.Redis)

			r := httptest.NewRequest(http.MethodPost, "/api/users/profile-pics", bytes.NewBuffer(tt.body))
			r.Header.Set("Content-Type", tt.contentType)
			r.Header.Set("Cookie", FmtCookie(TestSessionID2))
			r.Header.Set("Content-Digest", tt.checksum)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestHandleGetProfilePic(t *testing.T) {
	t.Parallel()

	profileKey1 := fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString())
	profileKey2 := fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString())

	tests := []struct {
		name        string
		userID      string
		wantStatus  int
		wantWithKey string
		setupMocks  func(*gomock.Controller) egress.S3ClientAPI
	}{
		{
			name:        "user has profile pic in storage",
			userID:      "1",
			wantStatus:  http.StatusTemporaryRedirect,
			wantWithKey: profileKey1, // expect to receive profileKey in the redirect response, since it is the latest uploaded picture
			setupMocks: func(ctrl *gomock.Controller) egress.S3ClientAPI {
				mockS3Client := egress.NewMockS3ClientAPI(ctrl)
				mockS3Client.EXPECT().
					ListObjectsV2(gomock.Any(), &s3.ListObjectsV2Input{
						Bucket: aws.String(egress.S3ProfileBucket),
						Prefix: aws.String("users/profile-pics/1"),
					}).
					Return(&s3.ListObjectsV2Output{
						// user has multiple profiles, with 'profileKey1' being the latest
						Contents: []s3Types.Object{
							{Key: aws.String(profileKey2), LastModified: aws.Time(time.Unix(1, 0))},
							{Key: aws.String(profileKey1), LastModified: aws.Time(time.Unix(2, 0))},
						},
					}, nil)
				return mockS3Client
			},
		},
		{
			name:       "user has default profile pic",
			userID:     "2",
			wantStatus: http.StatusOK,
			setupMocks: func(ctrl *gomock.Controller) egress.S3ClientAPI {
				mockS3Client := egress.NewMockS3ClientAPI(ctrl)
				mockS3Client.EXPECT().
					ListObjectsV2(gomock.Any(), &s3.ListObjectsV2Input{
						Bucket: aws.String(egress.S3ProfileBucket),
						Prefix: aws.String("users/profile-pics/2"),
					}).
					// no profile pics, send the default picture (200ok with a static image)
					Return(&s3.ListObjectsV2Output{}, nil)
				return mockS3Client
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := serviceMocks{
				Entropy:  &svc.StableEntropySource{},
				S3Client: tt.setupMocks(ctrl),
			}
			h, services := setupTestHandler(t, mocks, itest.Redis)
			defer services.Close()

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

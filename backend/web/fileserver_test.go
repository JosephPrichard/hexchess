package web

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"hexchess-svc/egress"
	"hexchess-svc/itest"
	"hexchess-svc/service"

	"hexchess-svc/util/testutil"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var s3InputCmpOpts = testutil.CmpIgnoreExcept(s3.PutObjectInput{}, "Bucket", "Key", "ContentType")

func TestHandleUploadProfilePic(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockS3Client := egress.NewMockS3Client(ctrl)
	mocks := svc.Mocks{
		Entropy:  &svc.StableEntropySource{},
		S3Client: mockS3Client,
	}

	services, _ := svc.SetupServicesTest(t, mocks, itest.Redis)
	defer services.Close()

	createTestSessions(t, services)

	strBody := "testfiledata"
	body := bytes.NewBuffer([]byte(strBody))

	// assert that keys and metadata arrive on the system correctly
	mockS3Client.EXPECT().
		PutObject(gomock.Any(), gomock.Cond(func(input *s3.PutObjectInput) bool {
			wantInput := &s3.PutObjectInput{
				Bucket:       aws.String(egress.S3ProfileBucket),
				Key:          aws.String("users/profile-pics/2/mock-1"),
				ContentType:  aws.String("application/octet-stream"),
				CacheControl: aws.String("public, max-age=31536000"),
			}
			recvBodyBytes, _ := io.ReadAll(input.Body)

			return testutil.Equal(t, wantInput, input, s3InputCmpOpts) && assert.Equal(t, strBody, string(recvBodyBytes))
		})).
		Return(&s3.PutObjectOutput{}, nil)

	// empty keylist, do not delete anything
	mockS3Client.EXPECT().
		ListObjectsV2(gomock.Any(), &s3.ListObjectsV2Input{
			Bucket: aws.String(egress.S3ProfileBucket),
			Prefix: aws.String("users/profile-pics/2"),
		}).
		Return(&s3.ListObjectsV2Output{}, nil)

	r := httptest.NewRequest(http.MethodPost, "/api/users/profile-pics", body)
	r.Header.Set("Content-Type", "application/octet-stream")
	r.Header.Set("Cookie", FmtCookie(TestSessionID2))
	w := httptest.NewRecorder()

	hander := MakeServeMux(Setup{Services: services})
	hander.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
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
		setupMocks  func(*gomock.Controller) egress.S3Client
	}{
		{
			name:        "user has profile pic in storage",
			userID:      "1",
			wantStatus:  http.StatusTemporaryRedirect,
			wantWithKey: profileKey1, // expect to receive profileKey in the redirect response, since it is the latest uploaded picture
			setupMocks: func(ctrl *gomock.Controller) egress.S3Client {
				mockS3Client := egress.NewMockS3Client(ctrl)
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
			setupMocks: func(ctrl *gomock.Controller) egress.S3Client {
				mockS3Client := egress.NewMockS3Client(ctrl)
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

			mocks := svc.Mocks{
				Entropy:  &svc.StableEntropySource{},
				S3Client: tt.setupMocks(ctrl),
			}

			services, _ := svc.SetupServicesTest(t, mocks, itest.Redis)
			defer services.Close()

			r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/users/profile-pics?userId=%s", tt.userID), nil)
			w := httptest.NewRecorder()

			hander := MakeServeMux(Setup{Services: services})
			hander.ServeHTTP(w, r)

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

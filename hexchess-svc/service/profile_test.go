package svc

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/mock/gomock"
	"testing"
	"time"

	"hexchess-svc/egress"
	"hexchess-svc/util/logutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteOldProfilePics(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockS3Client := egress.NewMockS3Client(ctrl)
	mocks := ServiceMocks{S3Client: mockS3Client}

	services, _ := SetupServicesTest(t, mocks)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	mockS3Client.EXPECT().
		ListObjectsV2(gomock.Any(), &s3.ListObjectsV2Input{
			Bucket: aws.String(egress.S3ProfileBucket),
			Prefix: aws.String("users/profile-pics/1"),
		}).
		Return(&s3.ListObjectsV2Output{
			Contents: []s3Types.Object{
				{Key: aws.String("users/profile-pics/1-1"), LastModified: aws.Time(time.Unix(1, 0))},
				{Key: aws.String("users/profile-pics/1-2"), LastModified: aws.Time(time.Unix(2, 0))},
				{Key: aws.String("users/profile-pics/1-3"), LastModified: aws.Time(time.Unix(3, 0))},
			},
		}, nil)

	mockS3Client.EXPECT().
		DeleteObjects(gomock.Any(), &s3.DeleteObjectsInput{
			Bucket: aws.String(egress.S3ProfileBucket),
			Delete: &s3Types.Delete{
				Objects: []s3Types.ObjectIdentifier{
					{Key: aws.String("users/profile-pics/1-1")},
					{Key: aws.String("users/profile-pics/1-2")},
				},
			},
		}).
		Return(&s3.DeleteObjectsOutput{}, nil)

	require.NoError(t, services.DeleteOldProfilePics(ctx, 1))
}

func TestFindMostRecentKey(t *testing.T) {
	t.Parallel()

	// tests most recent Key logic since it cannot be tested in the s3 calls it is tested in
	// this is because the 'LastModifiedTime' value is nondeterministic with regards to inserts that happen in +- 1 second
	tests := []struct {
		objects []s3Types.Object
		wantKey string
	}{
		{wantKey: ""},
		{
			objects: []s3Types.Object{
				{Key: aws.String("a"), LastModified: ptr(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
				{Key: aws.String("b"), LastModified: ptr(time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC))},
			},
			wantKey: "b",
		},
		{
			objects: []s3Types.Object{
				{Key: aws.String("b"), LastModified: ptr(time.Unix(1, 0))},
			},
			wantKey: "b",
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.wantKey, findMostRecentKey(tt.objects))
	}
}

func TestFilterLeastRecentKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		objects  []s3Types.Object
		wantKeys []s3Types.ObjectIdentifier
	}{
		{wantKeys: []s3Types.ObjectIdentifier{}},
		{
			objects:  []s3Types.Object{},
			wantKeys: []s3Types.ObjectIdentifier{},
		},
		{
			objects: []s3Types.Object{
				{Key: aws.String("b"), LastModified: ptr(time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC))},
				{Key: aws.String("a"), LastModified: ptr(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
			},
			wantKeys: []s3Types.ObjectIdentifier{
				{Key: aws.String("a")},
			},
		},
		{
			objects: []s3Types.Object{
				{Key: aws.String("b"), LastModified: ptr(time.Unix(1, 0))},
			},
			wantKeys: []s3Types.ObjectIdentifier{},
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.wantKeys, filterLeastRecentKeys(tt.objects))
	}
}

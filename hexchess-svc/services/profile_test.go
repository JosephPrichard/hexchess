package svc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"hexchess-svc/egress"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteOldProfilePics(t *testing.T) {
	t.Parallel()

	services := SetupServicesTest(t, itest.Aws)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	for _, user := range []string{"1", "1", "2", "2"} {
		egress.PutTestS3Object(t, services.AWS.S3Client, services.AWS.S3ProfileBucket, fmt.Sprintf("users/profile-pics/%s/%s", user, uuid.NewString()), []byte("testfiledat2"))
	}

	require.NoError(t, services.DeleteOldProfilePics(ctx, 1))

	// this test verifies that the function will always retain a single file per user, and that files for other users are not touched
	// we cannot verify which actual file is retainined because the uncertainty of LastModifiedTime is too high.
	assert.Equal(t, 2, egress.CountS3Objects(t, services.AWS.S3Client, services.AWS.S3ProfileBucket, "users/profile-pics/2"))
	assert.Equal(t, 1, egress.CountS3Objects(t, services.AWS.S3Client, services.AWS.S3ProfileBucket, "users/profile-pics/1"))
}

func TestFindMostRecentKey(t *testing.T) {
	t.Parallel()

	// tests most recent key logic since it cannot be tested in the s3 calls it is tested in
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

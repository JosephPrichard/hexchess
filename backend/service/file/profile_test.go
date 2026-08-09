package file

import (
	"bytes"
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
)

func setupProfileTest(t alog.TestLogger, flags ...itest.TestFlag) (*ProfileService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewProfileService(infra.AWS, async.AsyncDispatcher{}, entropy.RealSource{})

	return services, infra
}

func TestDeleteExpiredProfilePics(t *testing.T) {
	services, testinfra := setupProfileTest(t, itest.AWS)
	defer testinfra.Close()

	ctx := t.Context()

	profileKey1 := "users/profile-pics/1/1"
	profileKey2 := "users/profile-pics/1/2"
	lastProfileKey := "users/profile-pics/1/3"

	cloud.SetupS3Test(t, testinfra.AWS, []*s3.PutObjectInput{
		{
			Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
			Key:    aws.String(profileKey1),
			Body:   bytes.NewReader([]byte("test1")),
		},
		{
			Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
			Key:    aws.String(profileKey2),
			Body:   bytes.NewReader([]byte("test2")),
		},
		{
			Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
			Key:    aws.String(lastProfileKey),
			Body:   bytes.NewReader([]byte("test3")),
		},
	})

	require.NoError(t, services.deleteExpiredProfilePics(ctx, 1))

	objects, err := testinfra.AWS.S3Client.ListObjectsV2(t.Context(), &s3.ListObjectsV2Input{
		Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
	})
	if err != nil {
		t.Fatalf("failed to list profile pics: %v", err)
	}

	assert.Equal(t, 1, len(objects.Contents))
}

func TestFindMostRecentKey(t *testing.T) {
	// tests most recent Key logic since it cannot be tested in the s3 calls it is tested in
	// this is because the 'LastModifiedTime' value is nondeterministic with regards to inserts that happen in ~5 seconds
	tests := []struct {
		objects []s3Types.Object
		wantKey string
	}{
		{wantKey: ""},
		{
			objects: []s3Types.Object{
				{Key: aws.String("a"), LastModified: new(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
				{Key: aws.String("b"), LastModified: new(time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC))},
			},
			wantKey: "b",
		},
		{
			objects: []s3Types.Object{
				{Key: aws.String("b"), LastModified: new(time.Unix(1, 0))},
			},
			wantKey: "b",
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.wantKey, findMostRecentKey(tt.objects))
	}
}

func TestFilterLeastRecentKeys(t *testing.T) {
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
				{Key: aws.String("b"), LastModified: new(time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC))},
				{Key: aws.String("a"), LastModified: new(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
			},
			wantKeys: []s3Types.ObjectIdentifier{
				{Key: aws.String("a")},
			},
		},
		{
			objects: []s3Types.Object{
				{Key: aws.String("b"), LastModified: new(time.Unix(1, 0))},
			},
			wantKeys: []s3Types.ObjectIdentifier{},
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.wantKeys, filterLeastRecentKeys(tt.objects))
	}
}

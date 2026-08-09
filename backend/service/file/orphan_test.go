package file

import (
	"bytes"
	"fmt"
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/utils/alog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOrphanTest(t alog.TestLogger, flags ...itest.TestFlag) (*OrphanService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewOrphanService(infra.AWS, infra.Querier())

	return services, infra
}

func TestRemoveOrphanedBucketObjects(t *testing.T) {
	t.Parallel()

	services, testinfra := setupOrphanTest(t, itest.ROPostgres, itest.AWS)
	defer testinfra.Close()

	ctx := t.Context()

	profileKeyUserID1 := fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString())          // user exists in db
	profileKeyInvalidUserID := fmt.Sprintf("users/profile-pics/8000/%s", uuid.NewString()) // user does not exist in db

	cloud.SetupS3Test(t, testinfra.AWS, []*s3.PutObjectInput{
		{
			Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
			Key:    aws.String(profileKeyUserID1),
			Body:   bytes.NewReader([]byte("test1")),
		},
		{
			Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
			Key:    aws.String(profileKeyInvalidUserID),
			Body:   bytes.NewReader([]byte("test2")),
		},
	})

	err := services.ClearOrphanFiles(ctx, 1)
	require.NoError(t, err)

	objects, err := testinfra.AWS.S3Client.ListObjectsV2(t.Context(), &s3.ListObjectsV2Input{
		Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
	})
	if err != nil {
		t.Fatalf("failed to list profile pics: %v", err)
	}

	assert.Equal(t, 1, len(objects.Contents))
	assert.Equal(t, []string{profileKeyUserID1}, keysOfObjects(objects.Contents))
}

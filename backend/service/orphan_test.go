package svc

import (
	"bytes"
	"fmt"
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/lib/awsutils"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRemoveOrphanedBucketObjects(t *testing.T) {
	t.Parallel()

	services, testinfra := setupServicesTest(t, nil, itest.ROPostgres, itest.AWS)
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

	services.ClearOrphanFiles(ctx, 1)

	objects, err := testinfra.AWS.S3Client.ListObjectsV2(t.Context(), &s3.ListObjectsV2Input{
		Bucket: aws.String(testinfra.AWS.S3ProfileBucket),
	})
	if err != nil {
		t.Fatalf("failed to list profile pics: %v", err)
	}

	assert.Equal(t, 1, len(objects.Contents))
	assert.Equal(t, []string{profileKeyUserID1}, awsutils.KeysOfObjects(objects.Contents))
}

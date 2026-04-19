package svc

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/mock/gomock"
	"hexchess-svc/egress"

	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"

	"github.com/google/uuid"
)

func TestRemoveOrphanedBucketObjects(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockS3Client := egress.NewMockS3Client(ctrl)
	mocks := Mocks{S3Client: mockS3Client}

	services, _ := SetupServicesTest(t, mocks, itest.ROPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	profileKeyUserID1 := fmt.Sprintf("users/profile-pics/1/%s", uuid.NewString())
	profileKeyInvalidUserID := fmt.Sprintf("users/profile-pics/8000/%s", uuid.NewString())

	// suppose s3 produces the following key in the first page - we should NOT delete them since user with ID 1 exists in the DB
	mockS3Client.EXPECT().
		ListObjectsV2(
			gomock.Any(),
			&s3.ListObjectsV2Input{
				Bucket:  aws.String(egress.S3ProfileBucket),
				MaxKeys: aws.Int32(1),
				Prefix:  aws.String("users/profile-pics"),
			},
			gomock.Any()).
		Return(&s3.ListObjectsV2Output{
			Contents: []s3Types.Object{
				{Key: aws.String(profileKeyUserID1), LastModified: aws.Time(time.Unix(1, 0))},
			},
			// indicates to the s3 sdk that there is another page after this one.
			NextContinuationToken: aws.String("token"),
			IsTruncated:           aws.Bool(true),
		}, nil)

	// suppose s3 produces the following key in the second page - we delete it since it does not exist in the DB
	mockS3Client.EXPECT().
		ListObjectsV2(
			gomock.Any(),
			&s3.ListObjectsV2Input{
				Bucket:            aws.String(egress.S3ProfileBucket),
				MaxKeys:           aws.Int32(1),
				Prefix:            aws.String("users/profile-pics"),
				ContinuationToken: aws.String("token"), // token propagates
			},
			gomock.Any()).
		Return(&s3.ListObjectsV2Output{
			Contents: []s3Types.Object{
				{Key: aws.String(profileKeyInvalidUserID), LastModified: aws.Time(time.Unix(2, 0))},
			},
		}, nil)
	mockS3Client.EXPECT().
		DeleteObjects(gomock.Any(), &s3.DeleteObjectsInput{
			Bucket: aws.String(egress.S3ProfileBucket),
			Delete: &s3Types.Delete{Objects: []s3Types.ObjectIdentifier{
				{Key: aws.String(profileKeyInvalidUserID)},
			}},
		}).
		Return(&s3.DeleteObjectsOutput{}, nil)

	services.ClearBucketOrphans(ctx, 1)
}

package egress

import (
	"hexchess-svc/lib/testutil"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func SetupS3Test(t *testing.T, client AWSClient, objects []*s3.PutObjectInput) {
	for _, bucket := range []string{
		client.S3ProfileBucket,
	} {
		if _, err := client.S3Client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
			t.Fatalf("failed to create bucket: %v", err)
		}
	}
	for _, input := range objects {
		if _, err := client.S3Client.PutObject(t.Context(), input); err != nil {
			t.Fatalf("failed to upload profile pic: %v", err)
		}
	}
}

func MakeTestAWS() *AWSNames {
	return testutil.MakeTestNames(DefaultAWSNames)
}

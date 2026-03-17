package egress

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const S3ProfileBucket = "hexchess-profiles"

type AWS struct {
	S3ProfileBucket string
	S3Endpoint      string
	S3Client        *s3.Client
}

type AWSConfig struct {
	AWSDefaultRegion string
	AWSEndpoint      string
	IsLocalstack     bool
	S3ProfileBucket string
}

func MakeAwsClients(ctx context.Context, cfg AWSConfig) (AWS, error) {
	if cfg.S3ProfileBucket == "" {
		cfg.S3ProfileBucket = S3ProfileBucket
	}

	awsOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.AWSDefaultRegion),
	}
	if cfg.IsLocalstack {
		awsOpts = append(awsOpts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return AWS{}, fmt.Errorf("load aws config %+v: %w", cfg, err)
	}

	awsClient := AWS{
		S3ProfileBucket: cfg.S3ProfileBucket,
		S3Endpoint:      cfg.AWSEndpoint,
		S3Client: s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
			o.UsePathStyle = true
		}),
	}

	// creates all buckets by default.
	for _, bucket := range []string{
		cfg.S3ProfileBucket,
	} {
		if _, err := awsClient.S3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
			return awsClient, fmt.Errorf("create s3 bucket: %v: %s", bucket, err)
		}
	}

	return awsClient, nil
}

func (aws *AWS) MakeS3Url(bucket string, key string) string {
	return fmt.Sprintf("%s/%s/%s", aws.S3Endpoint, bucket, key)
}

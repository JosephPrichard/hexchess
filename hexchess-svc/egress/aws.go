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
	AWSSecretKey     string
	AWSSecretID      string
	AWSEndpoint      string

	S3ProfileBucket string
}

func MakeAwsClients(ctx context.Context, cfg AWSConfig) (AWS, error) {
	if cfg.S3ProfileBucket == "" {
		cfg.S3ProfileBucket = S3ProfileBucket
	}
	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.AWSDefaultRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AWSSecretKey, cfg.AWSSecretID, "")),
	)
	if err != nil {
		return AWS{}, fmt.Errorf("load aws config %+v: %w", cfg, err)
	}
	return AWS{
		S3ProfileBucket: cfg.S3ProfileBucket,
		S3Endpoint:      cfg.AWSEndpoint,
		S3Client: s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
			o.UsePathStyle = true
		}),
	}, nil
}

func (aws *AWS) MakeS3Url(bucket string, key string) string {
	return fmt.Sprintf("%s/%s/%s", aws.S3Endpoint, bucket, key)
}

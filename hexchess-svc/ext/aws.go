package ext

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Aws struct {
	S3ReplayBucket  string
	S3ProfileBucket string
	S3Endpoint      string
	S3Client        *s3.Client
}

type AwsConfig struct {
	AwsDefaultRegion string
	AwsSecretKey     string
	AwsSecretID      string
	AwsEndpoint      string

	S3ReplayBucket  string
	S3ProfileBucket string
}

func MakeAwsClients(ctx context.Context, cfg AwsConfig) (Aws, error) {
	if cfg.S3ReplayBucket == "" {
		cfg.S3ReplayBucket = S3ReplayBucket
	}
	if cfg.S3ProfileBucket == "" {
		cfg.S3ProfileBucket = S3ProfileBucket
	}
	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.AwsDefaultRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AwsSecretKey, cfg.AwsSecretID, "")),
	)
	if err != nil {
		return Aws{}, fmt.Errorf("load aws config %+v: %w", cfg, err)
	}
	return Aws{
		S3ReplayBucket:  cfg.S3ReplayBucket,
		S3ProfileBucket: cfg.S3ProfileBucket,
		S3Endpoint:      cfg.AwsEndpoint,
		S3Client: s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.AwsEndpoint)
			o.UsePathStyle = true
		}),
	}, nil
}

func (aws *Aws) MakeS3Url(bucket string, key string) string {
	return fmt.Sprintf("%s/%s/%s", aws.S3Endpoint, bucket, key)
}

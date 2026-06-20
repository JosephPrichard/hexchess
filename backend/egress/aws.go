package egress

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var DefaultAWSNames = AWSNames{
	S3ProfileBucket: "hexchess-profiles",
}

type AWSClient struct {
	AWSNames
	S3Endpoint string
	S3Client   *s3.Client
}

type AWSNames struct {
	S3ProfileBucket string
}

type AWSConfig struct {
	AWSDefaultRegion  string
	AWSEndpoint       string
	IsTestCredentials bool
}

func MakeAWSClients(ctx context.Context, cfg AWSConfig, names *AWSNames) (AWSClient, error) {
	awsOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.AWSDefaultRegion),
	}
	if cfg.IsTestCredentials {
		awsOpts = append(awsOpts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("testing", "testing", "")))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return AWSClient{}, fmt.Errorf("load aws config %+v: %w", cfg, err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
		o.UsePathStyle = true
	})

	if names == nil {
		names = &DefaultAWSNames
	}
	awsClient := AWSClient{
		S3Endpoint: cfg.AWSEndpoint,
		S3Client:   s3Client,
		AWSNames:   *names,
	}

	slog.Info("created aws client", "awsClient", awsClient, "awsCfg", awsCfg)

	go func() {
		buckets := []string{
			awsClient.S3ProfileBucket,
		}
		var errors []error
		for _, bucket := range buckets {
			if _, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
				errors = append(errors, err)
			}
		}
		if len(errors) > 0 {
			slog.Warn("failed to create buckets", "buckets", buckets, "errors", errors)
		} else {
			slog.Info("created buckets", "buckets", buckets)
		}
	}()

	return awsClient, nil
}

func (aws *AWSClient) MakeS3Url(bucket string, key string) string {
	return fmt.Sprintf("%s/%s/%s", aws.S3Endpoint, bucket, key)
}

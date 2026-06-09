package egress

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log/slog"
)

//go:generate mockgen -source=aws.go -destination=./aws_mock.go -package=egress

const S3ProfileBucket = "hexchess-profiles"

var Buckets = []string{S3ProfileBucket}

type S3ClientAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

type CompositeS3API struct {
	*s3.Client
	*s3.PresignClient
}

type AWS struct {
	S3Endpoint string
	S3Client   S3ClientAPI
}

type AWSConfig struct {
	AWSDefaultRegion string
	AWSEndpoint      string
	IsLocal          bool
}

func MakeAwsClients(ctx context.Context, cfg AWSConfig) (AWS, error) {
	awsOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.AWSDefaultRegion),
	}
	if cfg.IsLocal {
		awsOpts = append(awsOpts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return AWS{}, fmt.Errorf("load aws config %+v: %w", cfg, err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
		o.UsePathStyle = true
	})

	presignClient := s3.NewPresignClient(s3Client)
	awsClient := AWS{S3Endpoint: cfg.AWSEndpoint, S3Client: CompositeS3API{s3Client, presignClient}}

	// creates all buckets by default.
	go func() {
		for _, bucket := range Buckets {
			if _, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
				slog.WarnContext(ctx, "failed to create bucket", "bucket", bucket, "error", err)
			}
		}
	}()

	return awsClient, nil
}

func (aws *AWS) MakeS3Url(bucket string, key string) string {
	return fmt.Sprintf("%s/%s/%s", aws.S3Endpoint, bucket, key)
}

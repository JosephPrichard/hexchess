package cloud

import (
	"context"
	"fmt"
	"hexchess-svc/lib/logutil"
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

type AWSClientConfig struct {
	Names       *AWSNames
	Profile     string
	AWSRegion   string
	AWSEndpoint string
}

func NewAWSClients(ctx context.Context, clientCfg AWSClientConfig) AWSClient {
	awsOpts := []func(*config.LoadOptions) error{
		config.WithRegion(clientCfg.AWSRegion),
	}
	if clientCfg.Profile == "local" {
		awsOpts = append(awsOpts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("testing", "testing", "")))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		logutil.Fatal("load aws config", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(clientCfg.AWSEndpoint)
		o.UsePathStyle = true
	})

	if clientCfg.Names == nil {
		clientCfg.Names = &DefaultAWSNames
	}
	awsClient := AWSClient{
		S3Endpoint: clientCfg.AWSEndpoint,
		S3Client:   s3Client,
		AWSNames:   *clientCfg.Names,
	}

	slog.Info("created aws client", "awsClient", awsClient, "awsCfg", fmt.Sprintf("%+v", awsCfg))
	return awsClient
}

func (aws *AWSClient) NewS3Url(bucket string, key string) string {
	return fmt.Sprintf("%s/%s/%s", aws.S3Endpoint, bucket, key)
}

package cloud

import (
	"context"
	"fmt"
	"hexchess-svc/lib/config"
	"hexchess-svc/lib/logutil"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
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
	Names          *AWSNames      `json:"names"`
	ActiveProfile  config.Profile `json:"activeProfile"`
	AWSRegion      string         `json:"awsRegion"`
	AWSEndpoint    string         `json:"awsEndpoint"`
	StaticUsername string         `json:"staticUsername"`
	StaticPassword string         `json:"staticPassword"`
}

func NewAWSClients(ctx context.Context, clientCfg AWSClientConfig) AWSClient {
	awsOpts := []func(*awsConfig.LoadOptions) error{
		awsConfig.WithRegion(clientCfg.AWSRegion),
	}
	if clientCfg.ActiveProfile == config.Local {
		awsOpts = append(awsOpts, awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(clientCfg.StaticUsername, clientCfg.StaticPassword, "")))
	}
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsOpts...)
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

	slog.Info("created aws client", "config", clientCfg)
	return awsClient
}

func (aws *AWSClient) NewS3Url(bucket string, key string) string {
	return fmt.Sprintf("%s/%s/%s", aws.S3Endpoint, bucket, key)
}

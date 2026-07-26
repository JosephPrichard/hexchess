package cloud

import (
	"context"
	"fmt"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var DefaultAWSNames = AWSNames{
	S3ProfileBucket: "profiles",
}

type AWSClient struct {
	AWSNames
	S3Endpoint    string
	S3Client      *s3.Client
	PresignClient *s3.PresignClient
}

type AWSNames struct {
	S3ProfileBucket string
}

type AWSClientConfig struct {
	Names         AWSNames       `json:"names"`
	ActiveProfile config.Profile `json:"activeProfile"`
	AWSRegion     string         `json:"awsRegion"`
	AWSEndpoint   string         `json:"awsEndpoint"`
	AWSUsername   string         `json:"awsUsername"`
	AWSPassword   string         `json:"awsPassword"`
}

func NewAWSClients(ctx context.Context, cfg AWSClientConfig) AWSClient {
	awsOpts := []func(*awsConfig.LoadOptions) error{
		awsConfig.WithRegion(cfg.AWSRegion),
	}
	if cfg.ActiveProfile == config.Local {
		awsOpts = append(awsOpts, awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AWSUsername, cfg.AWSPassword, "")))
	}
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		logutil.Fatal("load aws config", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.AWSEndpoint != "" {
			o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
			o.UsePathStyle = true
		}
	})
	presignClient := s3.NewPresignClient(s3Client)

	awsClient := AWSClient{
		S3Endpoint:    cfg.AWSEndpoint,
		S3Client:      s3Client,
		PresignClient: presignClient,
		AWSNames:      cfg.Names,
	}

	slog.Info("created aws client", "config", cfg)
	return awsClient
}

func (aws *AWSClient) NewS3Url(bucket string, key string) string {
	return fmt.Sprintf("%s/%s/%s", aws.S3Endpoint, bucket, key)
}

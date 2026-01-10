package ext

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const S3Bucket = "hexchess-files"

type RemoteAPIs struct {
	GoogleAPI
}

func MakeRemoteAPIs() RemoteAPIs {
	return RemoteAPIs{
		GoogleAPI: &RemoteGoogleAPI{},
	}
}

type Aws struct {
	S3Bucket string
	S3Client *s3.Client
}

type AwsConfig struct {
	S3Bucket         string
	AwsDefaultRegion string
	AwsSecretKey     string
	AwsSecretID      string
	AwsEndpoint      string
}

func MakeAwsClients(ctx context.Context, cfg AwsConfig) (Aws, error) {
	if cfg.S3Bucket == "" {
		cfg.S3Bucket = S3Bucket
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
		S3Bucket: cfg.S3Bucket,
		S3Client: s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.AwsEndpoint)
			o.UsePathStyle = true
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		}),
	}, nil
}

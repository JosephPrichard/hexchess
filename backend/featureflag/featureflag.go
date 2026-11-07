package featureflag

import (
	"context"
	"hexchess-svc/config"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"time"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	ffclient "github.com/thomaspoignant/go-feature-flag"
	"github.com/thomaspoignant/go-feature-flag/ffcontext"
	"github.com/thomaspoignant/go-feature-flag/retriever/s3retrieverv2"
)

const (
	FeatureFlagBucket = "feature-flag-bucket"
	FeatureFlagItem   = "flags.yaml"

	ElasticSearchFlagKey = "elasticSearchEnabled"
)

var GlobalFlagUser = ffcontext.NewEvaluationContext("global")

type FeatureFlagAPI interface {
	Get(flagKey string, user ffcontext.Context) bool
}

type FFS3Client struct{}

func (*FFS3Client) Get(flagKey string, user ffcontext.Context) bool {
	flagValue, err := ffclient.BoolVariation(flagKey, user, false)
	if err != nil {
		slog.Error("feature flag: error retrieving flag, defaulting to false", "flagKey", flagKey)
	} else {
		slog.Info("feature flag: retrieved flag", "flagKey", flagKey, "flagValue", flagValue)
	}
	return flagValue
}

type FFStubClient struct {
	Store map[string]bool
}

func (client *FFStubClient) Get(flagKey string, _ ffcontext.Context) bool {
	if client.Store == nil {
		client.Store = make(map[string]bool)
	}
	if v, ok := client.Store[flagKey]; ok {
		return v
	}
	return false
}

func InitS3Client(ctx context.Context, activeProfile config.Profile, awsRegion string) {
	if activeProfile == config.Local {
		return
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(awsRegion))
	if err != nil {
		slogutil.Fatal("load aws config", err)
	}

	ffclient.Init(ffclient.Config{
		PollingInterval: 30 * time.Second,
		Retriever:       &s3retrieverv2.Retriever{Bucket: FeatureFlagBucket, Item: FeatureFlagItem, AwsConfig: &awsCfg},
	})
}

func Enabled(flagKey string, user ffcontext.EvaluationContext) bool {
	return (&FFS3Client{}).Get(flagKey, user)
}

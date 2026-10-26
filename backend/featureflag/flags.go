package featureflag

import (
	"encoding/json"
	"log/slog"
	"os"
)

// Config is a config that stores all the values of "static" feature flags
// a "static" feature flag is a flag whose value is determined at *build* time (changing a flag involves redeploying the app)
type Config struct {
	IsUserSearchElasticSearch bool `json:"isUserSearchElasticSearch"`
}

var FeatureFlagConfig Config

func init() {
	featureFlagConfigJson := os.Getenv("FEATURE_FLAGS")

	var flagConfig Config
	var err error

	if featureFlagConfigJson != "" {
		err = json.Unmarshal([]byte(featureFlagConfigJson), &flagConfig)
	}

	if err == nil {
		FeatureFlagConfig = flagConfig
		slog.Info("loaded feature flag configuration", "featureFlagConfigJson", featureFlagConfigJson, "loadedFlagConfig", flagConfig)
	} else {
		slog.Warn("failed to load feature flag configuration", "featureFlagConfigJson", featureFlagConfigJson)
	}
}

package config

// FeatureFlags is a config that stores all the values of "static" feature flags
// a "static" feature flag is a flag whose value is determined at *build* time (changing a flag involves redeploying the app)
type FeatureFlags struct {
	IsUserSearchElasticSearch bool `json:"isUserSearchElasticSearch"`
}

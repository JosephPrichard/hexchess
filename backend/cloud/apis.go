package cloud

import "net/http"

type SDKs struct {
	GoogleSDK
}

func NewRemoteAPIs(client *http.Client) SDKs {
	googleAPI, err := NewGoogleAPI("", client)
	if err != nil {
		panic(err)
	}
	return SDKs{GoogleSDK: googleAPI}
}

type RemoteAPIOpt func(*SDKs)

func WithGoogleAPI(googleAPI GoogleSDK) RemoteAPIOpt {
	return func(apis *SDKs) {
		apis.GoogleSDK = googleAPI
	}
}

func WithGoogleIDTokenValidator(validator GoogleTokenValidator, apiKey string) RemoteAPIOpt {
	return func(apis *SDKs) {
		apis.GoogleSDK = GoogleSDK{validator: validator, apiKey: apiKey}
	}
}

func NewOptRemoteAPIs(opts ...RemoteAPIOpt) SDKs {
	var apis SDKs
	for _, opt := range opts {
		opt(&apis)
	}
	return apis
}

package egress

import "net/http"

type RemoteAPIs struct {
	GoogleAPI
}

func MakeRemoteAPIs(client *http.Client) RemoteAPIs {
	googleAPI, err := MakeGoogleAPI("", client)
	if err != nil {
		panic(err)
	}
	return RemoteAPIs{GoogleAPI: googleAPI}
}

type RemoteAPIOpt func(*RemoteAPIs)

func WithGoogleAPI(googleAPI GoogleAPI) RemoteAPIOpt {
	return func(apis *RemoteAPIs) {
		apis.GoogleAPI = googleAPI
	}
}

func WithGoogleIDTokenValidator(validator GoogleTokenValidator, apiKey string) RemoteAPIOpt {
	return func(apis *RemoteAPIs) {
		apis.GoogleAPI = GoogleAPI{validator: validator, apiKey: apiKey}
	}
}

func MakeOptRemoteAPIs(opts ...RemoteAPIOpt) RemoteAPIs {
	var apis RemoteAPIs
	for _, opt := range opts {
		opt(&apis)
	}
	return apis
}

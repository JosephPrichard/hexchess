package egress

import (
	"context"
	"google.golang.org/api/idtoken"
	"hexchess-svc/lib/serrors"
	"net/http"
)

//go:generate mockgen -source=google.go -destination=./google_mock.go -package=egress

type IDTokenValidator interface {
	Validate(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error)
}

type GoogleAPI struct {
	APIKey    string
	Validator IDTokenValidator
}

func MakeGoogleAPI(apiKey string, client *http.Client) GoogleAPI {
	if client == nil {
		client = http.DefaultClient
	}
	validator, err := idtoken.NewValidator(context.Background(), idtoken.WithHTTPClient(client))
	if err != nil {
		panic(err)
	}
	return GoogleAPI{Validator: validator, APIKey: apiKey}
}

func MakeGoogleAPIWithValidator(apiKey string, validator IDTokenValidator) GoogleAPI {
	return GoogleAPI{APIKey: apiKey, Validator: validator}
}

type GoogleIDTokenPayload struct {
	AccountID string
	Username  string
}

const UsernameClaim string = "email"

func (google *GoogleAPI) ValidateGoogleIDToken(ctx context.Context, token string) (GoogleIDTokenPayload, error) {
	payload, err := google.Validator.Validate(ctx, token, google.APIKey)
	if err != nil {
		return GoogleIDTokenPayload{}, serrors.New("validate google id token", err, "token", token)
	}
	googleAccountID := payload.Subject
	username, ok := payload.Claims[UsernameClaim].(string)
	if !ok {
		return GoogleIDTokenPayload{}, serrors.New("validate google id token", err, "token", token, "payload", payload, "claim", UsernameClaim)
	}
	return GoogleIDTokenPayload{AccountID: googleAccountID, Username: username}, nil
}

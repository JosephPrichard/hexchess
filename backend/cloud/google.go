package cloud

import (
	"context"
	"hexchess-svc/utils/serrors"
	"net/http"

	"google.golang.org/api/idtoken"
)

//go:generate mockgen -source=google.go -destination=./google_mock.go -package=cloud

type GoogleTokenValidator interface {
	Validate(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error)
}

type GoogleAPI struct {
	apiKey    string
	validator GoogleTokenValidator
}

func NewGoogleAPI(apiKey string, client *http.Client) (GoogleAPI, error) {
	if client == nil {
		client = http.DefaultClient
	}
	validator, err := idtoken.NewValidator(context.Background(), idtoken.WithHTTPClient(client))
	if err != nil {
		return GoogleAPI{}, err
	}
	return GoogleAPI{validator: validator, apiKey: apiKey}, nil
}

type GoogleIDTokenResp struct {
	AccountID string
	Username  string
}

const UsernameClaim string = "email"

func (google *GoogleAPI) ValidateGoogleIDToken(ctx context.Context, token string) (GoogleIDTokenResp, error) {
	payload, err := google.validator.Validate(ctx, token, google.apiKey)
	if err != nil {
		return GoogleIDTokenResp{}, serrors.New("validate google id token", err, "token", token)
	}
	googleAccountID := payload.Subject
	username, ok := payload.Claims[UsernameClaim].(string)
	if !ok {
		return GoogleIDTokenResp{}, serrors.New("validate google id token", err, "token", token, "payload", payload, "claim", UsernameClaim)
	}
	return GoogleIDTokenResp{AccountID: googleAccountID, Username: username}, nil
}

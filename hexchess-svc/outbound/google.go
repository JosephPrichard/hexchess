package outbound

import (
	"context"
	"fmt"
	"google.golang.org/api/idtoken"
)

//go:generate mockgen -source=google.go -destination=./google_mock.go -package=ext

type GoogleAPI interface {
	ValidateIDToken(ctx context.Context, token string) (GoogleIDTokenPayload, error)
}

type RemoteGoogleAPI struct {
	APIKey string
}

type GoogleIDTokenPayload struct {
	AccountID string
	Username  string
}

const UsernameClaim string = "email"

func (g *RemoteGoogleAPI) ValidateIDToken(ctx context.Context, token string) (GoogleIDTokenPayload, error) {
	payload, err := idtoken.Validate(ctx, token, g.APIKey)
	if err != nil {
		return GoogleIDTokenPayload{}, fmt.Errorf("validate google id token: %w", err)
	}
	googleAccountID := payload.Subject
	username, ok := payload.Claims[UsernameClaim].(string)
	if !ok {
		return GoogleIDTokenPayload{}, fmt.Errorf("expected claim '%s' to be provided in payload: %v", UsernameClaim, payload)
	}
	return GoogleIDTokenPayload{AccountID: googleAccountID, Username: username}, nil
}

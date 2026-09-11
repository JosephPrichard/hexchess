package service

import (
	"context"
	"hexchess-svc/cloud"
)

type AuthTokenService struct {
	sdks cloud.SDKs
}

func NewAuthTokenService(sdks cloud.SDKs) *AuthTokenService {
	return &AuthTokenService{sdks}
}

func (services *AuthTokenService) ValidateGoogleIDToken(ctx context.Context, token string) (cloud.GoogleIDTokenResp, error) {
	return services.sdks.ValidateGoogleIDToken(ctx, token)
}

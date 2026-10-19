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

func (service *AuthTokenService) ValidateGoogleIDToken(ctx context.Context, token string) (cloud.GoogleIDTokenResp, error) {
	return service.sdks.ValidateGoogleIDToken(ctx, token)
}

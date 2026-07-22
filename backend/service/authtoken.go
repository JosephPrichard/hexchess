package svc

import (
	"context"
	"hexchess-svc/cloud"
)

func (services *HexchessServices) ValidateGoogleIDToken(ctx context.Context, token string) (cloud.GoogleIDTokenResp, error) {
	return services.sdks.ValidateGoogleIDToken(ctx, token)
}

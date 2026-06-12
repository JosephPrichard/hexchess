package svc

import (
	"context"
	"hexchess-svc/egress"
)

func (services *HexchessServices) ValidateGoogleIDToken(ctx context.Context, token string) (egress.GoogleIDTokenResp, error) {
	return services.remote.ValidateGoogleIDToken(ctx, token)
}

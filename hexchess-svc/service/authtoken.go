package svc

import (
	"context"
	"hexchess-svc/egress"
)

func (svc *HexchessServices) ValidateGoogleIDToken(ctx context.Context, token string) (egress.GoogleIDTokenPayload, error) {
	return svc.remote.ValidateGoogleIDToken(ctx, token)
}

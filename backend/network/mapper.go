package network

import (
	"hexchess-svc/game"
)

func mapChessMetas(metas []game.ChessMeta) []ChessMeta {
	result := make([]ChessMeta, 0, len(metas))

	for _, meta := range metas {
		result = append(result, ChessMeta{
			GameID:      meta.GameID,
			WhitePlayer: meta.WhitePlayer,
			BlackPlayer: meta.BlackPlayer,
			Mode:        meta.Mode.String(),
			Ordering:    meta.Ordering,
		})
	}

	return result
}

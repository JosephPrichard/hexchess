package network

import "hexchess-svc/model"

func mapChessMetas(metas []model.ChessMeta) []ChessMeta {
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

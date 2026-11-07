package userdata

import "github.com/jackc/pgx/v5/pgtype"

const StartElo float64 = 1000
const DefaultCountry = "un"

func DefaultUserElo(elo pgtype.Float8) float64 {
	if elo.Valid {
		return elo.Float64
	} else {
		return StartElo
	}
}

func CalcUserWinrate(wins int32, losses int32, draws int32) int64 {
	wr := float64(0)
	total := wins + losses + draws
	if total > 0 {
		wr = float64(wins) / float64(total) * 100.0
	}
	return int64(wr)
}

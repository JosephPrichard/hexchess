package domain

import (
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type User struct {
	ID       int64     `json:"id"`
	Username string    `json:"username"`
	Country  string    `json:"country"`
	Bio      string    `json:"bio"`
	JoinedOn time.Time `json:"joinedOn"`
}

type LbdUser struct {
	User
	Elo        float64 `json:"elo"`
	HighestElo float64 `json:"highestElo"`
	Wins       int32   `json:"wins"`
	Losses     int32   `json:"losses"`
	Draws      int32   `json:"draws"`
	Winrate    int64   `json:"winrate"`
	Rank       int64   `json:"rank"`
}

type ModeStats struct {
	Mode       GameMode `json:"mode"`
	Rank       int64    `json:"rank"`
	Wins       int32    `json:"wins"`
	Losses     int32    `json:"losses"`
	Draws      int32    `json:"draws"`
	Winrate    int64    `json:"winrate"`
	Elo        float64  `json:"elo"`
	HighestElo float64  `json:"highestElo"`
}

type UserStats struct {
	TotalWins    int32       `json:"totalWins"`
	TotalLosses  int32       `json:"totalLosses"`
	TotalDraws   int32       `json:"totalDraws"`
	AvgElo       float64     `json:"avgElo"`     // average elo of all other modes
	HighestElo   float64     `json:"highestElo"` // the absolute highest elo
	TotalWinrate int64       `json:"totalWinrate"`
	ModeStats    []ModeStats `json:"modeStats"`
}

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

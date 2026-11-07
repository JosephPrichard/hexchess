package user

import (
	"hexchess-svc/gamedata"
	"hexchess-svc/player"
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
	Mode       gamedata.GameMode `json:"mode"`
	Rank       int64             `json:"rank"`
	Wins       int32             `json:"wins"`
	Losses     int32             `json:"losses"`
	Draws      int32             `json:"draws"`
	Winrate    int64             `json:"winrate"`
	Elo        float64           `json:"elo"`
	HighestElo float64           `json:"highestElo"`
	Score      float64           `json:"-"`
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

type UpdateUserEvent struct {
	ID int64 `json:"id"`
}

func (p User) IsGuest() bool {
	return player.IsGuestID(p.ID)
}

func (p User) NonGuest() bool {
	return !player.IsGuestID(p.ID)
}

func (p User) IsSame(p1 User) bool {
	return p.ID == p1.ID
}

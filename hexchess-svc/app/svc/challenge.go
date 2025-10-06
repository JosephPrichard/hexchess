package svc

import "time"

type ChallengeRow struct {
	ChallengerId      int64
	ChallengerName    string
	ChallengerCountry string
	ChallengerElo     float32
	ChallengeeId      int64
	ChallengeeName    string
	ChallengeeCountry string
	ChallengeeElo     float32
	TimeControl       string
	StartColor        string // from challenger's perspective
	MadeOn            time.Time
}

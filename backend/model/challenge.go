package model

import "time"

type UserMessageKind string

const (
	ChallengeKind UserMessageKind = "challenge"
)

type UserMessageType struct {
	Kind UserMessageKind `json:"kind"`
}

type UserMessage struct {
	Kind      UserMessageKind `json:"kind"`
	Challenge Challenge       `json:"challenge"`
}

type Challenge struct {
	ChallengerID      int64     `json:"challengerId"`
	ChallengerName    string    `json:"challengerName"`
	ChallengerCountry string    `json:"challengerCountry"`
	ChallengerElo     float64   `json:"challengerElo"`
	ChallengeeID      int64     `json:"challengeeId"`
	ChallengeeName    string    `json:"challengeeName"`
	ChallengeeCountry string    `json:"challengeeCountry"`
	ChallengeeElo     float64   `json:"challengeeElo"`
	Mode              GameMode  `json:"mode"`
	StartColor        GameColor `json:"startColor"` // from challenger's perspective
	MadeOn            time.Time `json:"madeOn"`
	ExpiresOn         time.Time `json:"expiresOn"`
}

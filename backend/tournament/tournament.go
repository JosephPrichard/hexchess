package tournament

import (
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/replay"
	"hexchess-svc/user"
	"hexchess-svc/utils/opt"
	"time"

	"uuid"
)

type Tournament struct {
	ID                 int64             `json:"id"`
	TournamentKey      uuid.UUID         `json:"tournamentKey"`
	Name               string            `json:"name"`
	Rounds             int32             `json:"rounds"`
	WinnerID           int64             `json:"winnerId"`
	MaxPlayerCount     int               `json:"maxPlayerCount"`
	CountdownStartedOn time.Time         `json:"countdownStartedOn"`
	CountdownStarted   bool              `json:"isCountdownStarted"`
	Countdown          string            `json:"countdown"`
	CreatedOn          time.Time         `json:"createdOn"`
	CreatedBy          int64             `json:"createdBy"`
	Ruleset            TournamentRuleset `json:"ruleset"`
	Status             TournamentStatus  `json:"status"`
	Mode               gamedata.GameMode `json:"mode"`
}

type Participant = user.LbdUser

type TournamentReplay struct {
	replay.Replay
	replay.ReplayColorElos
}

type FullMatch struct {
	Ordering      int64                         `json:"ordering"`
	GameID        gameid.Identifier             `json:"gameId"`
	TournamentKey uuid.UUID                     `json:"tournamentKey"`
	Round         int32                         `json:"round"`
	CreatedOn     time.Time                     `json:"createdOn"`
	WhiteID       int64                         `json:"whiteId"`
	BlackID       int64                         `json:"blackId"`
	Replay        *TournamentReplay             `json:"replay"`
	GameMode      opt.Option[gamedata.GameMode] `json:"gameMode"` // sometimes initialized, contains the game mode of gameID
}

type MatchCreation struct {
	GameID   gameid.Identifier `json:"gameID"`
	GameMode gamedata.GameMode `json:"gameMode"`
	WhiteID  int64             `json:"whiteId"`
	BlackID  int64             `json:"blackId"`
}

type MatchCreations struct {
	Creations []FullMatch `json:"creations"`
}

type FullTournament struct {
	Participants []Participant `json:"participants"`
	Matches      []FullMatch   `json:"matches"`
	Tournament
}

type AdvanceTournamentEvent struct {
	EventID       uuid.UUID `json:"eventId"`
	TournamentKey uuid.UUID `json:"tournamentKey"`
}

type TournamentOutputKind string

const (
	TournamentParticipantKind TournamentOutputKind = "participant"
	TournamentCountdownKind   TournamentOutputKind = "countdown"
	TournamentStartKind       TournamentOutputKind = "start"
	TournamentMatchmakingKind TournamentOutputKind = "matchmaking"
	TournamentErrorKind       TournamentOutputKind = "error"
)

type TournamentOutput struct {
	Key             string               `json:"tournamentKey"`
	Kind            TournamentOutputKind `json:"kind"`
	Matches         []FullMatch          `json:"matches"`
	Error           string               `json:"error"`
	LeaderboardUser user.LbdUser         `json:"leaderboardUser"`
}

package queue

import (
	"hexchess-svc/chess"
	"hexchess-svc/gameid"
	"hexchess-svc/utils/opt"
	"time"

	"github.com/google/uuid"
)

type AdvanceTournamentJob struct {
	EventID       uuid.UUID `json:"eventId"`
	TournamentKey uuid.UUID `json:"tournamentKey"`
}

func (args AdvanceTournamentJob) Kind() string {
	return "advance_tournament"
}

type UpdateUserJob struct {
	ID int64 `json:"id"`
}

func (args UpdateUserJob) Kind() string {
	return "update_user"
}

type FinishGameJob struct {
	GameID       gameid.Identifier `json:"id"`
	InitialBoard chess.Board       `json:"initialBoard"`
	Moves        []chess.HistMove  `json:"moves"`
	WhiteID      int64             `json:"whiteID"`
	BlackID      int64             `json:"blackID"`
	ReplayMode   string            `json:"mode"`
	ReplayResult string            `json:"replayResult"`
	ReplayCause  string            `json:"replayCause"`
	InsertedTime time.Time         `json:"insertedTime"`
}

type UpdtGameMetadataJob struct {
	GameID      gameid.Identifier `json:"id"`
	WhitePlayer opt.Option[int64] `json:"whitePlayer"`
	BlackPlayer opt.Option[int64] `json:"blackPlayer"`
	Mode        string            `json:"mode"`
	FirstColor  string            `json:"firstColor"`
}

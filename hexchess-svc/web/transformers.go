package web

import (
	"hexchess-svc/hexchess"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"hexchess-svc/util/enum"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Action int

const (
	Accept Action = iota
	Reject
	Delete
)

type UpdateChallengeTBody struct {
	ChallengeeID int64
	ChallengerID int64
	TargetID     int64
	Action       Action
}

func transformUpdateChallenge(body UpdateChallengeBody) (UpdateChallengeTBody, error) {
	var action Action
	switch strings.ToUpper(body.Action) {
	case "ACCEPT":
		action = Accept
	case "REJECT":
		action = Reject
	case "DELETE":
		action = Delete
	default:
		return UpdateChallengeTBody{}, OneRespError("action", ErrHttpInvalidAction)
	}

	var targetID int64
	switch action {
	case Accept, Reject:
		// reject, accept means challenge is directed at challengee
		targetID = body.ChallengeeID
	default:
		// delete means challenge is directed at challenger
		targetID = body.ChallengerID
	}

	return UpdateChallengeTBody{
		ChallengeeID: body.ChallengeeID,
		ChallengerID: body.ChallengerID,
		TargetID:     targetID,
		Action:       action,
	}, nil
}

type CreateChallengeTBody struct {
	ChallengeeID int64
	StartColor   model.GameColor
	Mode         model.GameMode
}

func transformCreateChallenge(body CreateChallengeBody) (CreateChallengeTBody, error) {
	var respErr ResponseError

	color, ok := enum.Parse(body.StartColor, model.GameColorEnums)
	if !ok {
		respErr.Put("startColor", ErrHttpInvalidColor)
	}
	mode, ok := enum.Parse(body.Mode, model.GameModeEnums)
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}

	tbody := CreateChallengeTBody{ChallengeeID: body.ChallengeeID, StartColor: color, Mode: mode}
	return tbody, respErr.AsError()
}

type CreateGameTBody struct {
	FirstColor   model.GameColor
	Mode         model.GameMode
	InitialBoard hexchess.Board
}

func transformCreateGame(body CreateGameBody) (CreateGameTBody, error) {
	initialBoard := hexchess.InitialBoard()
	var respErr ResponseError

	if body.InitialFEN != "" {
		board, err := hexchess.ParseFen(body.InitialFEN)
		if err != nil {
			respErr.Put("initialFen", ErrHttpInvalidFen)
		} else {
			initialBoard = board
		}
	}

	color, ok := enum.Parse(body.FirstColor, model.GameColorEnums)
	if !ok {
		respErr.Put("firstColor", ErrHttpInvalidColor)
	}
	mode, ok := enum.Parse(body.Mode, model.GameModeEnums)
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}

	tbody := CreateGameTBody{FirstColor: color, Mode: mode, InitialBoard: initialBoard}
	return tbody, respErr.AsError()
}

type CreateTournamentTBody struct {
	Name      string
	Mode      model.GameMode
	Ruleset   model.TournamentRuleset
	Rounds    int32
	Countdown time.Duration
}

func transformCreateTournament(body CreateTournamentBody) (CreateTournamentTBody, error) {
	var respErr ResponseError

	mode, ok := enum.Parse(body.Mode, model.GameModeEnums)
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}
	ruleset, ok := enum.Parse(body.Ruleset, model.TournamentRulesetEnums)
	if !ok {
		respErr.Put("ruleset", ErrHttpInvalidRuleset)
	}

	tbody := CreateTournamentTBody{Name: body.Name, Mode: mode, Ruleset: ruleset, Rounds: body.Rounds, Countdown: body.Countdown}
	return tbody, nil
}

type JoinTournamentTBody struct {
	TournamentKey uuid.UUID
}

func transformTournamentKey(body TournamentKeyBody) (tbody JoinTournamentTBody, err error) {
	tkey, err := uuid.Parse(body.TournamentKey)
	if err != nil {
		return tbody, OneRespError("tournamentKey", ErrHttpInvalidID)
	}
	return JoinTournamentTBody{TournamentKey: tkey}, err
}

type ChessMetasQuery struct {
	Page  int
	Count int
}

func transformChessMetasQuery(values url.Values) (ChessMetasQuery, error) {
	var respErr ResponseError

	page, err := intQueryDefault(values, "page", 1)
	if err != nil {
		respErr.Put("page", ErrHttpInvalidPage)
	}
	count, err := intQueryDefault(values, "count", perPage)
	if err != nil {
		respErr.Put("count", ErrHttpInvalidCount)
	}

	query := ChessMetasQuery{Page: page, Count: count}
	return query, respErr.AsError()
}

const (
	ByReplayID = "BY_REPLAY_ID"
	ByGameID   = "BY_GAME_ID"
)

type GetReplayQuery struct {
	ReplayID  int64
	GameID    string
	HasGameID bool
}

func transformReplayQuery(values url.Values) (GetReplayQuery, error) {
	var respErr ResponseError

	kindStr := values.Get("idKind")
	if kindStr == "" {
		kindStr = ByReplayID
	}

	var replayID int64
	var gameID string
	var hasGameID bool

	switch kindStr {
	case ByReplayID:
		intID, err := strconv.Atoi(values.Get("id"))
		if err != nil {
			respErr.Put("id", ErrHttpInvalidID)
		}
		replayID = int64(intID)
	case ByGameID:
		gameID = values.Get("id")
		hasGameID = true
	}

	query := GetReplayQuery{ReplayID: replayID, GameID: gameID, HasGameID: hasGameID}
	return query, respErr.AsError()
}

var timeframeMap = map[string]uint{
	"1m":  1,
	"3m":  3,
	"6m":  6,
	"1y":  12,
	"all": 0,
}

type EloHistoriesQuery struct {
	UserID int
	Months uint
}

func transformEloHistoriesQuery(values url.Values) (EloHistoriesQuery, error) {
	var respErr ResponseError

	userID, err := strconv.Atoi(values.Get("userId"))
	if err != nil {
		respErr.Put("userId", ErrHttpInvalidID)
	}

	months, ok := timeframeMap[queryDefault(values, "timeframe", "all")]
	if !ok {
		respErr.Put("timeframe", ErrHttpInvalidTimeframe)
	}

	query := EloHistoriesQuery{UserID: userID, Months: months}
	return query, respErr.AsError()
}

type GetReplaysQuery = svc.ReplayQuery

func transformReplaysQuery(values url.Values) (GetReplaysQuery, error) {
	var respErr ResponseError

	userID, err := intQueryDefault(values, "userId", -1)
	if err != nil {
		respErr.Put("userId", ErrHttpInvalidID)
	}
	winnerID, err := intQueryDefault(values, "winnerId", -1)
	if err != nil {
		respErr.Put("userId", ErrHttpInvalidID)
	}
	loserID, err := intQueryDefault(values, "loserId", -1)
	if err != nil {
		respErr.Put("userId", ErrHttpInvalidID)
	}
	afterID, err := intQueryDefault(values, "afterId", -1)
	if err != nil {
		respErr.Put("afterId", ErrHttpInvalidID)
	}

	query := GetReplaysQuery{
		UserID:   int64(userID),
		WinnerID: int64(winnerID),
		LoserID:  int64(loserID),
		AfterID:  int64(afterID),
	}
	return query, respErr.AsError()
}

type GetTournamentQuery struct {
	UserID        int
	AfterID       int
	ByParticipant bool
}

func transformTournamentsQuery(values url.Values) (q GetTournamentQuery, err error) {
	var respErr ResponseError

	userIDStr := values.Get("userId")
	userID := svc.NoParticipantSignifier

	if userIDStr != "" {
		userID, err = strconv.Atoi(userIDStr)
		if err != nil {
			respErr.Put("userId", ErrHttpInvalidID)
		}
	}
	afterID, err := strconv.Atoi(values.Get("afterId"))
	if err != nil {
		respErr.Put("afterId", ErrHttpInvalidID)
	}

	query := GetTournamentQuery{UserID: userID, AfterID: afterID}
	return query, respErr.AsError()
}

func transformLeaderboardQuery(q url.Values) (LeaderboardQuery, error) {
	var respErr ResponseError

	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		respErr.Put("page", ErrHttpInvalidPage)
	}
	mode, ok := enum.Parse(q.Get("mode"), model.GameModeEnums)
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}

	query := LeaderboardQuery{Page: page, Mode: mode}
	return query, respErr.AsError()
}

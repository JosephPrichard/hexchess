package web

import (
	"context"
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

	const startColorKey = "startColor"
	const modeKey = "mode"

	color, ok := enum.Parse(body.StartColor, model.GameColorEnums)
	if !ok {
		respErr.Put(startColorKey, ErrHttpInvalidColor)
	}
	mode, ok := enum.Parse(body.Mode, model.GameModeEnums)
	if !ok {
		respErr.Put(modeKey, ErrHttpInvalidMode)
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

	const initialFenKey = "initialFen"
	const modeKey = "mode"
	const firstColorKey = "firstColor"

	if body.InitialFEN != "" {
		board, err := hexchess.ParseFen(body.InitialFEN)
		if err != nil {
			respErr.Put(initialFenKey, ErrHttpInvalidFen)
		} else {
			initialBoard = board
		}
	}

	color, ok := enum.Parse(body.FirstColor, model.GameColorEnums)
	if !ok {
		respErr.Put(firstColorKey, ErrHttpInvalidColor)
	}
	mode, ok := enum.Parse(body.Mode, model.GameModeEnums)
	if !ok {
		respErr.Put(modeKey, ErrHttpInvalidMode)
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

	const modeKey = "mode"
	const rulesetKey = "ruleset"

	mode, ok := enum.Parse(body.Mode, model.GameModeEnums)
	if !ok {
		respErr.Put(modeKey, ErrHttpInvalidMode)
	}
	ruleset, ok := enum.Parse(body.Ruleset, model.TournamentRulesetEnums)
	if !ok {
		respErr.Put(rulesetKey, ErrHttpInvalidRuleset)
	}

	tbody := CreateTournamentTBody{Name: body.Name, Mode: mode, Ruleset: ruleset, Rounds: body.Rounds, Countdown: body.Countdown}
	return tbody, nil
}

type JoinTournamentTBody struct {
	TournamentKey uuid.UUID
}

func transformTournamentKey(body TournamentKeyBody) (tbody JoinTournamentTBody, err error) {
	const tournamentKeyKey = "tournamentKey"

	tkey, err := uuid.Parse(body.TournamentKey)
	if err != nil {
		return tbody, OneRespError(tournamentKeyKey, ErrHttpInvalidID)
	}
	return JoinTournamentTBody{TournamentKey: tkey}, err
}

type ChessMetasQuery struct {
	Page  int
	Count int
}

func transformChessMetasQuery(values url.Values) (ChessMetasQuery, error) {
	var respErr ResponseError

	const pageKey = "page"
	const countKey = "count"

	page, err := parseDefaultInt(values, pageKey, 1)
	if err != nil {
		respErr.Put(pageKey, ErrHttpInvalidPage)
	}
	count, err := parseDefaultInt(values, countKey, perPage)
	if err != nil {
		respErr.Put(countKey, ErrHttpInvalidCount)
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

	const idKey = "id"
	const idKindKey = "idKind"

	kindStr := values.Get(idKindKey)
	if kindStr == "" {
		kindStr = ByReplayID
	}

	var replayID int64
	var gameID string
	var hasGameID bool

	switch kindStr {
	case ByReplayID:
		intID, err := strconv.Atoi(values.Get(idKey))
		if err != nil {
			respErr.Put(idKey, ErrHttpInvalidID)
		}
		replayID = int64(intID)
	case ByGameID:
		gameID = values.Get(idKey)
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

	const userIDKey = "userId"
	const timeframeKey = "timeframe"

	userID, err := strconv.Atoi(values.Get(userIDKey))
	if err != nil {
		respErr.Put(userIDKey, ErrHttpInvalidID)
	}

	months, ok := timeframeMap[parseDefaultString(values, timeframeKey, "all")]
	if !ok {
		respErr.Put(timeframeKey, ErrHttpInvalidTimeframe)
	}

	query := EloHistoriesQuery{UserID: userID, Months: months}
	return query, respErr.AsError()
}

type GetReplaysQuery = svc.ReplayQuery

func transformReplaysQuery(ctx context.Context, values url.Values) (GetReplaysQuery, error) {
	var respErr ResponseError

	const userIDKey = "userId"
	const winnerIDKey = "winnerId"
	const loserIDKey = "loserId"
	const whiteIDKey = "whiteId"
	const blackIDKey = "blackId"
	const modeKey = "mode"
	const resultKey = "result"
	const causeKey = "cause"
	const fromDateKey = "fromDate"
	const toDateKey = "toDate"
	const whiteNameKey = "whitename"
	const blackNameKey = "blackname"
	const loserNameKey = "losername"
	const winnerNameKey = "winnername"

	const afterIDKey = "afterId"
	const afterRatingKey = "afterRating"
	const afterTurnCountKey = "afterTurnCount"

	const sortKey = "sort"

	// parsing search conditions
	whiteName := parseOptionalString(values, whiteNameKey)
	blackName := parseOptionalString(values, blackNameKey)
	loserName := parseOptionalString(values, loserNameKey)
	winnerName := parseOptionalString(values, winnerNameKey)

	userID := parseOptionalInt(values, userIDKey, &respErr, ErrHttpInvalidID)
	winnerID := parseOptionalInt(values, winnerIDKey, &respErr, ErrHttpInvalidID)
	loserID := parseOptionalInt(values, loserIDKey, &respErr, ErrHttpInvalidID)
	whiteID := parseOptionalInt(values, whiteIDKey, &respErr, ErrHttpInvalidID)
	blackID := parseOptionalInt(values, blackIDKey, &respErr, ErrHttpInvalidID)

	mode, ok := enum.ParseOptional(values.Get(modeKey), model.GameModeEnums)
	if !ok {
		respErr.Put(modeKey, ErrHttpInvalidMode)
	}
	result, ok := enum.ParseOptional(values.Get(resultKey), model.ReplayResultEnums)
	if !ok {
		respErr.Put(resultKey, ErrHttpInvalidMode)
	}
	cause, ok := enum.ParseOptional(values.Get(causeKey), model.ReplayCauseEnums)
	if !ok {
		respErr.Put(causeKey, ErrHttpInvalidMode)
	}

	fromDate := parseOptionalDatetime(ctx, values, toDateKey, &respErr)
	toDate := parseOptionalDatetime(ctx, values, fromDateKey, &respErr)

	// parsing sort cursors
	afterID := parseOptionalInt(values, afterIDKey, &respErr, ErrHttpInvalidID)
	afterTurnCount := parseOptionalInt(values, afterTurnCountKey, &respErr, ErrHttpInvalidID)
	afterRating := parseOptionalFloat(values, afterRatingKey, &respErr)

	// parsing sort enum
	sort, ok := enum.ParseDefault(values.Get(sortKey), svc.ReplayQuerySortEnums, svc.ReplaySortID)
	if !ok {
		respErr.Put(sortKey, ErrHttpInvalidReplaySort)
	}

	query := GetReplaysQuery{
		WhiteName:      whiteName,
		BlackName:      blackName,
		LoserName:      loserName,
		WinnerName:     winnerName,
		UserID:         userID,
		WinnerID:       winnerID,
		LoserID:        loserID,
		WhiteID:        whiteID,
		BlackID:        blackID,
		Mode:           mode,
		Result:         result,
		Cause:          cause,
		FromDate:       fromDate,
		ToDate:         toDate,
		AfterID:        afterID,
		AfterTurnCount: afterTurnCount,
		AfterRating:    afterRating,
		Sort:           sort,
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

	const userIDKey = "userId"
	const afterIDKey = "afterId"

	userID, err := parseDefaultInt(values, userIDKey, svc.NoParticipantSignifier)
	if err != nil {
		respErr.Put(userIDKey, ErrHttpInvalidID)
	}
	afterID, err := strconv.Atoi(values.Get(afterIDKey))
	if err != nil {
		respErr.Put(afterIDKey, ErrHttpInvalidID)
	}

	query := GetTournamentQuery{UserID: userID, AfterID: afterID}
	return query, respErr.AsError()
}

func transformLeaderboardQuery(q url.Values) (LeaderboardQuery, error) {
	var respErr ResponseError

	const pageKey = "page"
	const modeKey = "mode"

	page, err := parseDefaultInt(q, pageKey, 1)
	if err != nil {
		respErr.Put(pageKey, ErrHttpInvalidPage)
	}
	mode, ok := enum.Parse(q.Get(modeKey), model.GameModeEnums)
	if !ok {
		respErr.Put(modeKey, ErrHttpInvalidMode)
	}

	query := LeaderboardQuery{Page: page, Mode: mode}
	return query, respErr.AsError()
}

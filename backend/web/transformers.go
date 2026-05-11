package web

import (
	"fmt"

	"hexchess-svc/internal/enum"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"
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
		return UpdateChallengeTBody{}, oneRespError("action", fmt.Errorf("invalid value: %s", body.Action))
	}

	var targetID int64
	switch action {
	case Accept, Reject:
		targetID = body.ChallengeeID
	default:
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

	color, err := enum.Parse(body.StartColor, model.GameColorEnums)
	if err != nil {
		respErr.Put("startColor", BadRequestError{err})
	}
	mode, err := enum.Parse(body.Mode, model.GameModeEnums)
	if err != nil {
		respErr.Put("mode", BadRequestError{err})
	}

	return CreateChallengeTBody{ChallengeeID: body.ChallengeeID, StartColor: color, Mode: mode}, respErr.AsError()
}

type CreateGameTBody struct {
	FirstColor   model.GameColor
	Mode         model.GameMode
	InitialBoard chess.Board
}

func transformCreateGame(body CreateGameBody) (CreateGameTBody, error) {
	var respErr ResponseError

	initialBoard := chess.InitialBoard()
	if body.InitialFEN != "" {
		parsedBoard, err := chess.ParseFen(body.InitialFEN)
		if err == nil {
			initialBoard = parsedBoard
		} else {
			respErr.Put("initialFen", BadRequestError{err})
		}
	}

	color, err := enum.Parse(body.FirstColor, model.GameColorEnums)
	if err != nil {
		respErr.Put("firstColor", BadRequestError{err})
	}
	mode, err := enum.Parse(body.Mode, model.GameModeEnums)
	if err != nil {
		respErr.Put("mode", BadRequestError{err})
	}

	return CreateGameTBody{FirstColor: color, Mode: mode, InitialBoard: initialBoard}, respErr.AsError()
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

	mode, err := enum.Parse(body.Mode, model.GameModeEnums)
	if err != nil {
		respErr.Put("mode", BadRequestError{err})
	}
	ruleset, err := enum.Parse(body.Ruleset, model.TournamentRulesetEnums)
	if err != nil {
		respErr.Put("ruleset", BadRequestError{err})
	}

	return CreateTournamentTBody{Name: body.Name, Mode: mode, Ruleset: ruleset, Rounds: body.Rounds, Countdown: body.Countdown}, respErr.AsError()
}

type ChessMetasQuery struct {
	Page  int
	Count int
}

func transformChessMetasQuery(values url.Values) (ChessMetasQuery, error) {
	ctx := MakeQueryParseCtx(values)

	page := parseDefaultInt(ctx, "page", 1)
	count := parseDefaultInt(ctx, "count", defaultPaginationCount)

	return ChessMetasQuery{Page: page, Count: count}, ctx.RespErr.AsError()
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
	ctx := MakeQueryParseCtx(values)

	kindStr := values.Get("idKind")
	if kindStr == "" {
		kindStr = ByReplayID
	}

	var replayID int64
	var gameID string
	var hasGameID bool

	switch kindStr {
	case ByReplayID:
		replayID = int64(parseInt(ctx, "id"))
		hasGameID = false
	case ByGameID:
		gameID = values.Get("id")
		hasGameID = true
	}

	return GetReplayQuery{ReplayID: replayID, GameID: gameID, HasGameID: hasGameID}, ctx.RespErr.AsError()
}

var timeframeMap = map[string]uint{
	"1m":  1,
	"3m":  3,
	"6m":  6,
	"1y":  12,
	"all": 0,
}

var InvalidTimeframeError = fmt.Errorf("invalid timeframe: expected one of %v", slices.Collect(maps.Keys(timeframeMap)))

type EloHistoriesQuery struct {
	UserID int
	Months uint
}

func transformEloHistoriesQuery(values url.Values) (EloHistoriesQuery, error) {
	ctx := MakeQueryParseCtx(values)

	userID := parseInt(ctx, "userId")

	months, ok := timeframeMap[parseDefaultString(values, "timeframe", "all")]
	if !ok {
		ctx.RespErr.Put("timeframe", BadRequestError{InvalidTimeframeError})
	}

	return EloHistoriesQuery{UserID: userID, Months: months}, ctx.RespErr.AsError()
}

type GetReplaysQuery = svc.ReplaysQuery

func transformReplaysQuery(values url.Values) (GetReplaysQuery, error) {
	ctx := MakeQueryParseCtx(values)

	whiteName := parseOptString(ctx, "whitename")
	blackName := parseOptString(ctx, "blackname")
	loserName := parseOptString(ctx, "losername")
	winnerName := parseOptString(ctx, "winnername")

	userID := parseOptInt[int64](ctx, "userId")
	winnerID := parseOptInt[int64](ctx, "winnerId")
	loserID := parseOptInt[int64](ctx, "loserId")
	whiteID := parseOptInt[int64](ctx, "whiteId")
	blackID := parseOptInt[int64](ctx, "blackId")

	mode := parseOptEnum(ctx, "mode", model.GameModeEnums)
	result := parseOptEnum(ctx, "result", model.ReplayResultEnums)
	cause := parseOptEnum(ctx, "cause", model.ReplayCauseEnums)

	fromDate := parseOptDatetime(ctx, "fromDate")
	toDate := parseOptDatetime(ctx, "toDate")

	afterID := parseOptInt[int64](ctx, "afterId")
	afterTurnCount := parseOptInt[int32](ctx, "afterTurnCount")
	afterRating := parseOptFloat(ctx, "afterRating")

	sort := parseDefEnum(ctx, "sort", svc.ReplayQuerySortEnums, svc.ReplaySortID)

	perPage := parseDefaultInt[int32](ctx, "perPage", defaultPaginationCount)
	if perPage > defaultPaginationCount {
		perPage = defaultPaginationCount
	}

	return GetReplaysQuery{
		WhiteName:  whiteName,
		BlackName:  blackName,
		WinnerName: winnerName,
		LoserName:  loserName,

		UserID:   userID,
		WinnerID: winnerID,
		LoserID:  loserID,
		WhiteID:  whiteID,
		BlackID:  blackID,

		Mode:     mode,
		Result:   result,
		Cause:    cause,
		FromDate: fromDate,
		ToDate:   toDate,

		AfterID:        afterID,
		AfterTurnCount: afterTurnCount,
		AfterRating:    afterRating,

		Sort:    sort,
		PerPage: perPage,
	}, ctx.RespErr.AsError()
}

type GetTournamentQuery struct {
	UserID        int
	AfterID       int
	ByParticipant bool
}

func transformTournamentsQuery(values url.Values) (GetTournamentQuery, error) {
	ctx := MakeQueryParseCtx(values)

	userID := parseDefaultInt(ctx, "userId", svc.NoParticipantSignifier)
	afterID := parseInt(ctx, "afterId")

	return GetTournamentQuery{UserID: userID, AfterID: afterID}, ctx.RespErr.AsError()
}

func transformLeaderboardQuery(values url.Values) (LeaderboardQuery, error) {
	ctx := MakeQueryParseCtx(values)

	page := parseDefaultInt(ctx, "page", svc.NoParticipantSignifier)
	mode := parseEnum(ctx, "mode", model.GameModeEnums)

	return LeaderboardQuery{Page: page, Mode: mode}, ctx.RespErr.AsError()
}

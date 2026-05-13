package web

import (
	"fmt"
	"github.com/google/uuid"
	"hexchess-svc/chess"

	"hexchess-svc/internal/enum"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"net/url"
	"strings"
	"time"
)

const (
	minPasswordLength = 11
	minUsernameLength = 5
	maxUsernameLength = 35
	maxBioLength      = 500
)

func isPasswordValid(password string) bool {
	return len(password) >= minPasswordLength
}

func validateRegisterBody(body RegisterBody) error {
	var respErr ResponseError
	if !isPasswordValid(body.Password) {
		respErr.Put("password", ErrHttpInvalidPassword)
	}
	if body.Password != body.ConfirmPassword {
		respErr.Put("confirmPassword", ErrHttpConfirmPassword)
	}
	if !isUsernameValid(body.Username) {
		respErr.Put("username", ErrHttpInvalidUsername)
	}
	return respErr.Inner()
}

func isUsernameValid(username string) bool {
	return len(username) >= minUsernameLength && len(username) <= maxUsernameLength
}

func validateUpdatePasswordBody(body UpdatePasswordBody) error {
	var respErr ResponseError
	if !isPasswordValid(body.NewPassword) {
		respErr.Put("newPassword", ErrHttpInvalidPassword)
	}
	if body.NewPassword != body.ConfirmNewPassword {
		respErr.Put("confirmNewPassword", ErrHttpConfirmPassword)
	}
	return respErr.Inner()
}

func validateUpdateUserBody(static StaticData, body UpdateUserBody) error {
	var respErr ResponseError
	if body.NewUsername != "" {
		if !isUsernameValid(body.NewUsername) {
			respErr.Put("newUsername", ErrHttpInvalidUsername)
		}
	}
	if body.NewBio != "" {
		if len(body.NewBio) > maxBioLength {
			respErr.Put("newBio", ErrHttpInvalidBio)
		}
	}
	if body.NewCountry != "" {
		if !static.validCountries[body.NewCountry] {
			respErr.Put("newCountry", ErrHttpInvalidCountry)
		}
	}
	return respErr.Inner()
}

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

func parseUpdateChallengeBody(body UpdateChallengeBody) (UpdateChallengeTBody, error) {
	var action Action
	switch strings.ToUpper(body.Action) {
	case "ACCEPT":
		action = Accept
	case "REJECT":
		action = Reject
	case "DELETE":
		action = Delete
	default:
		return UpdateChallengeTBody{}, respError("action", fmt.Errorf("invalid value: %s", body.Action))
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

func parseCreateChallengeBody(body CreateChallengeBody) (CreateChallengeTBody, error) {
	var respErr ResponseError

	color, err := enum.Parse(body.StartColor, model.GameColorEnums)
	if err != nil {
		respErr.Put("startColor", BadRequestError{err})
	}
	mode, err := enum.Parse(body.Mode, model.GameModeEnums)
	if err != nil {
		respErr.Put("mode", BadRequestError{err})
	}

	return CreateChallengeTBody{ChallengeeID: body.ChallengeeID, StartColor: color, Mode: mode}, respErr.Inner()
}

type CreateGameTBody struct {
	FirstColor   model.GameColor
	Mode         model.GameMode
	InitialBoard chess.Board
}

func parseCreateGameBody(body CreateGameBody) (CreateGameTBody, error) {
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

	return CreateGameTBody{FirstColor: color, Mode: mode, InitialBoard: initialBoard}, respErr.Inner()
}

type CreateTournamentTBody struct {
	Name      string
	Mode      model.GameMode
	Ruleset   model.TournamentRuleset
	Rounds    int32
	Countdown time.Duration
}

func parseCreateTournamentBody(body CreateTournamentBody) (CreateTournamentTBody, error) {
	var respErr ResponseError

	mode, err := enum.Parse(body.Mode, model.GameModeEnums)
	if err != nil {
		respErr.Put("mode", BadRequestError{err})
	}
	ruleset, err := enum.Parse(body.Ruleset, model.TournamentRulesetEnums)
	if err != nil {
		respErr.Put("ruleset", BadRequestError{err})
	}

	return CreateTournamentTBody{Name: body.Name, Mode: mode, Ruleset: ruleset, Rounds: body.Rounds, Countdown: body.Countdown}, respErr.Inner()
}

type ChessMetasQuery struct {
	Page  int
	Count int
}

func parseChessMetasQuery(values url.Values) (ChessMetasQuery, error) {
	ctx := MakeQueryParseCtx(values)

	page := parseDefaultInt(ctx, "page", 1)
	count := parseDefaultInt(ctx, "count", defaultPaginationCount)

	return ChessMetasQuery{Page: page, Count: count}, ctx.RespErr.Inner()
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

func parseReplayQueryBody(values url.Values) (GetReplayQuery, error) {
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

	return GetReplayQuery{ReplayID: replayID, GameID: gameID, HasGameID: hasGameID}, ctx.RespErr.Inner()
}

type TimeframeKind int

const (
	Timeframe1m TimeframeKind = iota
	Timeframe3m
	Timeframe6m
	Timeframe1y
	TimeframeAll
)

func (t TimeframeKind) Months() uint {
	switch t {
	case Timeframe1m:
		return 1
	case Timeframe3m:
		return 3
	case Timeframe6m:
		return 6
	case Timeframe1y:
		return 12
	case TimeframeAll:
		return 0
	}
	return 0
}

var timeframeEntries = []enum.Entry[TimeframeKind]{
	{Enum: Timeframe1m, String: "1m"},
	{Enum: Timeframe3m, String: "3m"},
	{Enum: Timeframe6m, String: "6m"},
	{Enum: Timeframe1y, String: "1y"},
	{Enum: TimeframeAll, String: "all"},
}

var TimeframeEnums = enum.BuildReverseMap(timeframeEntries)

func (t TimeframeKind) String() string {
	return enum.String(t, timeframeEntries)
}

type EloHistoriesQuery struct {
	UserID int
	Months uint
}

func parseEloHistoriesQuery(values url.Values) (EloHistoriesQuery, error) {
	ctx := MakeQueryParseCtx(values)

	userID := parseInt(ctx, "userId")
	timeframeKind := parseDefEnum[TimeframeKind](ctx, "timeframe", TimeframeEnums, TimeframeAll)

	return EloHistoriesQuery{UserID: userID, Months: timeframeKind.Months()}, ctx.RespErr.Inner()
}

type GetReplaysQuery = svc.ReplaysQuery

func parseReplaysQuery(values url.Values) (GetReplaysQuery, error) {
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
	}, ctx.RespErr.Inner()
}

type GetTournamentQuery struct {
	UserID        enum.Optional[int64]
	AfterID       enum.Optional[int64]
	ByParticipant bool
}

func parseTournamentsQuery(values url.Values) (GetTournamentQuery, error) {
	ctx := MakeQueryParseCtx(values)

	userID := parseOptInt[int64](ctx, "userId")
	afterID := parseOptInt[int64](ctx, "afterId")

	return GetTournamentQuery{UserID: userID, AfterID: afterID}, ctx.RespErr.Inner()
}

func parseLeaderboardQuery(values url.Values) (LeaderboardQuery, error) {
	ctx := MakeQueryParseCtx(values)

	page := parseDefaultInt(ctx, "page", 1)
	mode := parseEnum(ctx, "mode", model.GameModeEnums)

	return LeaderboardQuery{Page: page, Mode: mode}, ctx.RespErr.Inner()
}

func parseTournamentKeyBody(body TournamentKeyBody) (uuid.UUID, error) {
	tournamentKey, err := uuid.Parse(body.TournamentKey)
	if err != nil {
		return uuid.UUID{}, respError("tournamentKey", BadRequestError{err})
	}
	return tournamentKey, nil
}

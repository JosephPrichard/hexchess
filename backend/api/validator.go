package api

import (
	"errors"
	"fmt"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	"github.com/google/uuid"
	"hexchess-svc/assets"
	"hexchess-svc/chess"
	"strconv"
	"sync"

	"hexchess-svc/lib/enum"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"net/url"
	"strings"
	"time"
)

var (
	validate   *validator.Validate
	translator ut.Translator
	once       sync.Once
)

func makeEnumValidator(allowed []string) validator.Func {
	set := make(map[string]struct{}, len(allowed))
	for _, v := range allowed {
		set[v] = struct{}{}
	}
	return func(fl validator.FieldLevel) bool {
		_, ok := set[fl.Field().String()]
		return ok
	}
}

func GetValidator() *validator.Validate {
	once.Do(func() {
		validate = validator.New()
		validate.RegisterValidation("countries", makeEnumValidator(assets.GetCountryList()))
	})

	locale := en.New()
	uni := ut.New(locale, locale)
	t, _ := uni.GetTranslator("en")

	translator = t

	enTranslations.RegisterDefaultTranslations(validate, translator)

	return validate
}

func doValidation[Data any](data *Data) error {
	if err := GetValidator().Struct(data); err != nil {
		var respErr BadRequestError

		errs, ok := err.(validator.ValidationErrors)
		if ok {
			for _, e := range errs {
				respErr.Put(e.Namespace(), errors.New(e.Translate(translator)))
			}
			return respErr.Inner()
		} else {
			return err
		}
	}
	return nil
}

var ValidationFieldErrorMap = map[string]error{
	"RegisterBody.Password":          ErrHttpInvalidPassword,
	"RegisterBody.Username":          ErrHttpInvalidUsername,
	"UpdateUserBody.NewBio":          ErrHttpInvalidBio,
	"UpdateUserBody.NewCountry":      ErrHttpInvalidCountry,
	"UpdateUserBody.NewUsername":     ErrHttpInvalidUsername,
	"UpdatePasswordBody.NewPassword": ErrHttpInvalidPassword,
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
		return UpdateChallengeTBody{}, respError("UpdateChallengeBody.Action", fmt.Errorf("invalid value: %s", body.Action))
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
	var respErr BadRequestError

	color, err := enum.Parse(body.StartColor, model.GameColorEnums)
	if err != nil {
		respErr.Put("CreateChallengeBody.StartColor", err)
	}
	mode, err := enum.Parse(body.Mode, model.GameModeEnums)
	if err != nil {
		respErr.Put("CreateChallengeBody.Mode", err)
	}

	return CreateChallengeTBody{ChallengeeID: body.ChallengeeID, StartColor: color, Mode: mode}, respErr.Inner()
}

type CreateGameTBody struct {
	FirstColor   model.GameColor
	Mode         model.GameMode
	InitialBoard chess.Board
}

func parseCreateGameBody(body CreateGameBody) (CreateGameTBody, error) {
	var respErr BadRequestError

	initialBoard := chess.InitialBoard()
	if body.InitialFEN != "" {
		parsedBoard, err := chess.ParseFen(body.InitialFEN)
		if err == nil {
			initialBoard = parsedBoard
		} else {
			respErr.Put("CreateGameBody.InitialFen", err)
		}
	}

	color, err := enum.Parse(body.FirstColor, model.GameColorEnums)
	if err != nil {
		respErr.Put("CreateGameBody.FirstColor", err)
	}
	mode, err := enum.Parse(body.Mode, model.GameModeEnums)
	if err != nil {
		respErr.Put("CreateGameBody.Mode", err)
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
	var respErr BadRequestError

	mode, err := enum.Parse(body.Mode, model.GameModeEnums)
	if err != nil {
		respErr.Put("CreateTournamentBody.Mode", err)
	}
	ruleset, err := enum.Parse(body.Ruleset, model.TournamentRulesetEnums)
	if err != nil {
		respErr.Put("CreateTournamentBody.Ruleset", err)
	}

	return CreateTournamentTBody{Name: body.Name, Mode: mode, Ruleset: ruleset, Rounds: body.Rounds, Countdown: body.Countdown}, respErr.Inner()
}

type ChessMetasQuery struct {
	AfterOrdering enum.Optional[int64]
	Count         int32
}

func parseChessMetasQuery(values url.Values) (ChessMetasQuery, error) {
	q := makeQueryParseCtx(values)

	afterOrdering := parseOptInt[int64](q, "afterOrdering")
	count := parseDefaultInt(q, "count", defaultPaginationCount)

	return ChessMetasQuery{AfterOrdering: afterOrdering, Count: int32(count)}, q.RespErr.Inner()
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
	q := makeQueryParseCtx(values)

	kindStr := values.Get("idKind")
	if kindStr == "" {
		kindStr = ByReplayID
	}

	var replayID int64
	var gameID string
	var hasGameID bool

	switch kindStr {
	case ByReplayID:
		replayID = int64(parseInt(q, "id"))
		hasGameID = false
	case ByGameID:
		gameID = values.Get("id")
		hasGameID = true
	}

	return GetReplayQuery{ReplayID: replayID, GameID: gameID, HasGameID: hasGameID}, q.RespErr.Inner()
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
	q := makeQueryParseCtx(values)

	userID := parseInt(q, "userId")
	timeframeKind := parseDefEnum[TimeframeKind](q, "timeframe", TimeframeEnums, TimeframeAll)

	return EloHistoriesQuery{UserID: userID, Months: timeframeKind.Months()}, q.RespErr.Inner()
}

type GetReplaysQuery = svc.ReplaysQuery

func parseReplaysQuery(values url.Values) (GetReplaysQuery, error) {
	q := makeQueryParseCtx(values)

	whiteName := parseOptString(q, "whitename")
	blackName := parseOptString(q, "blackname")
	loserName := parseOptString(q, "losername")
	winnerName := parseOptString(q, "winnername")

	userID := parseOptInt[int64](q, "userId")
	winnerID := parseOptInt[int64](q, "winnerId")
	loserID := parseOptInt[int64](q, "loserId")
	whiteID := parseOptInt[int64](q, "whiteId")
	blackID := parseOptInt[int64](q, "blackId")

	mode := parseOptEnum(q, "mode", model.GameModeEnums)
	result := parseOptEnum(q, "result", model.ReplayResultEnums)
	cause := parseOptEnum(q, "cause", model.ReplayCauseEnums)

	fromDate := parseOptDatetime(q, "fromDate")
	toDate := parseOptDatetime(q, "toDate")

	afterID := parseOptInt[int64](q, "afterId")
	afterTurnCount := parseOptInt[int32](q, "afterTurnCount")
	afterRating := parseOptFloat(q, "afterRating")

	sort := parseDefEnum(q, "sort", svc.ReplayQuerySortEnums, svc.ReplaySortID)

	perPage := parseDefaultInt[int32](q, "perPage", defaultPaginationCount)

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
	}, q.RespErr.Inner()
}

type GetTournamentQuery struct {
	UserID        enum.Optional[int64]
	AfterID       enum.Optional[int64]
	ByParticipant bool
}

func parseTournamentsQuery(values url.Values) (GetTournamentQuery, error) {
	ctx := makeQueryParseCtx(values)

	userID := parseOptInt[int64](ctx, "userId")
	afterID := parseOptInt[int64](ctx, "afterId")

	return GetTournamentQuery{UserID: userID, AfterID: afterID}, ctx.RespErr.Inner()
}

func parseLeaderboardQuery(values url.Values) (LeaderboardQuery, error) {
	ctx := makeQueryParseCtx(values)

	page := parseDefaultInt(ctx, "page", 1)
	mode := parseEnum(ctx, "mode", model.GameModeEnums)

	return LeaderboardQuery{Page: page, Mode: mode}, ctx.RespErr.Inner()
}

func parseTournamentKeyBody(body TournamentKeyBody) (uuid.UUID, error) {
	tournamentKey, err := uuid.Parse(body.TournamentKey)
	if err != nil {
		return uuid.UUID{}, respError("TournamentKeyBody.TournamentKey", err)
	}
	return tournamentKey, nil
}

type queryParseCtx struct {
	Values  url.Values
	RespErr *BadRequestError
}

func makeQueryParseCtx(values url.Values) queryParseCtx {
	return queryParseCtx{Values: values, RespErr: &BadRequestError{}}
}

func parseInt(q queryParseCtx, key string) int {
	str := q.Values.Get(key)
	v, err := strconv.Atoi(str)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return v
}

func parseDefaultInt[T interface{ int | int32 | int64 }](q queryParseCtx, key string, def T) T {
	str := q.Values.Get(key)
	if str == "" {
		return def
	}
	v, err := strconv.Atoi(str)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return T(v)
}

func parseOptFloat(q queryParseCtx, key string) enum.Optional[float64] {
	v := q.Values.Get(key)
	if v == "" {
		return enum.Optional[float64]{}
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return enum.Just(f)
}

func parseOptString(q queryParseCtx, key string) enum.Optional[string] {
	v := q.Values.Get(key)
	if v == "" {
		return enum.Optional[string]{}
	}
	return enum.Just(v)
}

func parseOptDatetime(q queryParseCtx, key string) enum.Optional[time.Time] {
	v := q.Values.Get(key)
	if v == "" {
		return enum.Optional[time.Time]{}
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return enum.Just(t)
}

func parseOptInt[T interface{ int | int32 | int64 }](q queryParseCtx, key string) enum.Optional[T] {
	v := q.Values.Get(key)
	if v == "" {
		return enum.Optional[T]{}
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return enum.Just(T(i))
}

func parseOptEnum[T ~int](q queryParseCtx, key string, enums map[string]T) enum.Optional[T] {
	v, err := enum.ParseOptional(q.Values.Get(key), enums)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return v
}

func parseEnum[T ~int](q queryParseCtx, key string, enums map[string]T) T {
	v, err := enum.Parse(q.Values.Get(key), enums)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return v
}

func parseDefEnum[T ~int](q queryParseCtx, key string, enums map[string]T, def T) T {
	v, err := enum.ParseDefault(q.Values.Get(key), enums, def)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return v
}

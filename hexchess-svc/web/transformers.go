package web

import (
	"hexchess-svc/chess"
	svc "hexchess-svc/service"
	"net/url"
	"strconv"
	"strings"
)

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

func transformCreateChallenge(body CreateChallengeBody) (CreateChallengeTBody, error) {
	var respErr ResponseError
	color, ok := svc.GameColorEnums[body.StartColor]
	if !ok {
		respErr.Put("startColor", ErrHttpInvalidColor)
	}
	mode, ok := svc.GameModeEnums[body.Mode]
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}
	tbody := CreateChallengeTBody{ChallengeeID: body.ChallengeeID, StartColor: color, Mode: mode}
	return tbody, respErr.AsError()
}

func transformCreateGame(body CreateGameBody) (CreateGameTBody, error) {
	initialBoard := chess.InitialBoard()
	var respErr ResponseError

	if body.InitialFEN != "" {
		board, err := chess.ParseFen(body.InitialFEN)
		if err != nil {
			respErr.Put("initialFen", ErrHttpInvalidFen)
		} else {
			initialBoard = board
		}
	}
	color, ok := svc.GameColorEnums[body.FirstColor]
	if !ok {
		respErr.Put("firstColor", ErrHttpInvalidColor)
	}
	mode, ok := svc.GameModeEnums[body.Mode]
	if !ok {
		respErr.Put("mode", ErrHttpInvalidMode)
	}
	if respErr.HasErrors() {
		return CreateGameTBody{}, respErr.AsError()
	}

	tbody := CreateGameTBody{FirstColor: color, Mode: mode, InitialBoard: initialBoard}
	return tbody, nil
}

func (api *API) transformChessMetasQuery(q url.Values) (ChessMetasQuery, error) {
	var respErr ResponseError
	page, err := intQueryDefault(q, "page", 1)
	if err != nil {
		respErr.Put("page", ErrHttpInvalidPage)
	}
	count, err := intQueryDefault(q, "count", perPage)
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

func (api *API) transformReplayQuery(q url.Values) (GetReplayQuery, error) {
	var respErr ResponseError

	kindStr := q.Get("idKind")
	if kindStr == "" {
		kindStr = ByReplayID
	}

	var replayID int64
	var gameID string
	var hasGameID bool

	switch kindStr {
	case ByReplayID:
		intID, err := strconv.Atoi(q.Get("id"))
		if err != nil {
			respErr.Put("id", ErrHttpInvalidID)
		}
		replayID = int64(intID)
	case ByGameID:
		gameID = q.Get("id")
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

func (api *API) transformEloHistoriesQuery(q url.Values) (EloHistoriesQuery, error) {
	var respErr ResponseError
	userID, err := strconv.Atoi(q.Get("userId"))
	if err != nil {
		respErr.Put("userID", ErrHttpInvalidID)
	}

	months, ok := timeframeMap[queryDefault(q, "timeframe", "all")]
	if !ok {
		respErr.Put("timeframe", ErrHttpInvalidTimeframe)
	}

	query := EloHistoriesQuery{UserID: userID, Months: months}
	return query, respErr.AsError()
}

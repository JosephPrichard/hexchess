package replay

import (
	"context"
	"errors"
	"hexchess-svc/database"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/optional"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ReplaySearchService struct {
	database.Operator
}

func NewSearchService(operator database.Operator) *ReplaySearchService {
	return &ReplaySearchService{Operator: operator}
}

type ReplaysQuery struct {
	WhiteName  optional.Option[string] `json:"whiteName"`
	BlackName  optional.Option[string] `json:"blackName"`
	WinnerName optional.Option[string] `json:"winnerName"`
	LoserName  optional.Option[string] `json:"loserName"`

	UserID   optional.Option[int64] `json:"userId"`
	WhiteID  optional.Option[int64] `json:"whiteId"`
	BlackID  optional.Option[int64] `json:"blackId"`
	LoserID  optional.Option[int64] `json:"loserID"`
	WinnerID optional.Option[int64] `json:"winnerId"`

	Result   optional.Option[model.ReplayResult] `json:"result"`
	Mode     optional.Option[model.GameMode]     `json:"mode"`
	Cause    optional.Option[model.ReplayCause]  `json:"cause"`
	FromDate optional.Option[time.Time]          `json:"fromDate"`
	ToDate   optional.Option[time.Time]          `json:"toDate"`

	AfterID        optional.Option[int64]   `json:"afterId"`
	AfterRating    optional.Option[float64] `json:"afterRating"`
	AfterTurnCount optional.Option[int32]   `json:"afterTurnCount"`

	Sort ReplayQuerySortKey

	PerPage int32
}

type ReplayQuerySortKey int

const (
	ReplaySortID ReplayQuerySortKey = iota
	ReplaySortRating
	ReplaySortTurnCount
)

var replayQuerySortKeyEntries = []enum.Entry[ReplayQuerySortKey]{
	{Enum: ReplaySortID, String: "id"},
	{Enum: ReplaySortRating, String: "rating"},
	{Enum: ReplaySortTurnCount, String: "turnCount"},
}

var ReplayQuerySortEnums = enum.BuildReverseMap(replayQuerySortKeyEntries)

func (r ReplayQuerySortKey) String() string { return enum.String(r, replayQuerySortKeyEntries) }

func (services *ReplaySearchService) SearchReplaysByQuery(ctx context.Context, qry ReplaysQuery) ([]model.FullReplay, error) {
	defer perf.WithContext(ctx).Log()

	// get any userIDs requested through the username queries
	byNameRequests := []IDByNameRequest{
		{UsernameInput: qry.WhiteName, UserIDOutput: &qry.WhiteID},
		{UsernameInput: qry.BlackName, UserIDOutput: &qry.BlackID},
		{UsernameInput: qry.LoserName, UserIDOutput: &qry.LoserID},
		{UsernameInput: qry.WinnerName, UserIDOutput: &qry.WinnerID},
	}
	err := services.getIDsByUsernames(ctx, byNameRequests)
	if errors.Is(err, errUserNotFound) {
		// if any username cannot be matched to an userID, the search query will never yield any replays
		return []model.FullReplay{}, nil
	} else if err != nil {
		return nil, err
	}

	afterID := qry.AfterID.OrElse(math.MaxInt64)
	afterTurnCount := qry.AfterTurnCount.OrElse(math.MaxInt32)
	afterRating := qry.AfterRating.OrElse(math.MaxFloat64)

	// uses the unix epoch in days for range queries on date. this truncates away timestamp precision regarding hours, seconds, etc.
	fromDateDays := optional.Option[int32]{Value: timeutil.ToDayEpoch(qry.FromDate.Value), Present: qry.FromDate.Present}
	toDateDays := optional.Option[int32]{Value: timeutil.ToDayEpoch(qry.ToDate.Value), Present: qry.ToDate.Present}

	params := query.SelectReplaysByQueryParams{
		PerPage: qry.PerPage,

		// search constraints with mixed 'OR' 'AND' constraints
		UserID:       database.MapOptInt8(qry.UserID),
		WhiteID:      database.MapOptInt8(qry.WhiteID),
		BlackID:      database.MapOptInt8(qry.BlackID),
		WinnerID:     database.MapOptInt8(qry.WinnerID),
		LoserID:      database.MapOptInt8(qry.LoserID),
		Mode:         database.MapOptMode(qry.Mode),
		Result:       database.MapOptResult(qry.Result),
		Cause:        database.MapOptCause(qry.Cause),
		FromDateDays: database.MapOptInt4(fromDateDays),
		ToDateDays:   database.MapOptInt4(toDateDays),

		// search cursor used for pagination, afterID is always provided on a cursor search, rating and turnCount are only provided with sort
		AfterID:        afterID,
		AfterRating:    pgtype.Float8{Float64: afterRating, Valid: true},
		AfterTurnCount: afterTurnCount,

		// sort determines the 'ORDER BY' in the SQL query
		SortKey: qry.Sort.String(),
	}
	replayRows, err := services.Querier.SelectReplaysByQuery(ctx, params)
	if err != nil {
		return nil, serrors.New("select replays by query", err)
	}

	replays := make([]model.FullReplay, 0, len(replayRows))
	for _, row := range replayRows {
		replays = append(replays, mapFullReplayByIDRow(query.SelectReplayByIDRow(row)))
	}

	slog.InfoContext(ctx, "selected replays", "replaysQuery", qry, "replays", replays)
	return replays, nil
}

type IDByNameRequest struct {
	UsernameInput optional.Option[string] // if not provided, the search query will be ignored
	UserIDOutput  *optional.Option[int64] // if provided, this output is ignored
}

var errUserNotFound = errors.New("user not found")

func (services *ReplaySearchService) getIDsByUsernames(ctx context.Context, requests []IDByNameRequest) error {
	var usernames []string
	for _, request := range requests {
		if !request.UsernameInput.Present || request.UserIDOutput.Present {
			continue
		}
		usernames = append(usernames, request.UsernameInput.Value)
	}
	if len(usernames) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "selecting user ids by usernames for requests", "requests", requests)

	userRows, err := services.Querier.SelectUserIDsByNames(ctx, usernames)
	if err != nil {
		return serrors.New("select user ids by names", err, "usernames", usernames)
	}

	userIDs := make(map[string]int64)
	for _, row := range userRows {
		userIDs[row.Username] = row.ID
	}

	for _, request := range requests {
		if !request.UsernameInput.Present {
			continue
		}
		userID, exists := userIDs[request.UsernameInput.Value]
		if !exists {
			return errUserNotFound
		}
		*request.UserIDOutput = optional.Some(userID)
	}
	return nil
}

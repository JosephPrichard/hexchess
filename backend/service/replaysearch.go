package service

import (
	"context"
	"errors"
	"hexchess-svc/database"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ReplaySearchService struct {
	database.Database
}

func NewReplaySearchService(database database.Database) *ReplaySearchService {
	return &ReplaySearchService{Database: database}
}

type ReplaysQuery struct {
	WhiteName  opt.Option[string] `json:"whiteName"`
	BlackName  opt.Option[string] `json:"blackName"`
	WinnerName opt.Option[string] `json:"winnerName"`
	LoserName  opt.Option[string] `json:"loserName"`

	UserID   opt.Option[int64] `json:"userId"`
	WhiteID  opt.Option[int64] `json:"whiteId"`
	BlackID  opt.Option[int64] `json:"blackId"`
	LoserID  opt.Option[int64] `json:"loserID"`
	WinnerID opt.Option[int64] `json:"winnerId"`

	Result   opt.Option[model.ReplayResult] `json:"result"`
	Mode     opt.Option[model.GameMode]     `json:"mode"`
	Cause    opt.Option[model.ReplayCause]  `json:"cause"`
	FromDate opt.Option[time.Time]          `json:"fromDate"`
	ToDate   opt.Option[time.Time]          `json:"toDate"`

	AfterID        opt.Option[int64]   `json:"afterId"`
	AfterRating    opt.Option[float64] `json:"afterRating"`
	AfterTurnCount opt.Option[int32]   `json:"afterTurnCount"`

	Sort ReplayQuerySortKey `json:"sort"`

	PerPage int32 `json:"perPage"`
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

func (sort ReplayQuerySortKey) String() string { return enum.String(sort, replayQuerySortKeyEntries) }

func (service *ReplaySearchService) SearchReplaysByQuery(ctx context.Context, searchQuery ReplaysQuery) (_ []model.FullReplay, err error) {
	defer perf.WithContext(ctx).LogFailable(err)

	// populate any userIDs requested through the username queries
	err = service.loadUserIDsByUsernames(ctx, &searchQuery)
	if errors.Is(err, errUserNotFound) {
		// if any username cannot be matched to an userID, the search query will never yield any replays
		return []model.FullReplay{}, nil
	} else if err != nil {
		return nil, err
	}

	afterIDDB := int64(math.MaxInt64)
	afterTurnCountDB := int32(math.MaxInt32)
	afterRatingDB := math.MaxFloat64

	if searchQuery.AfterID.Present {
		afterIDDB = searchQuery.AfterID.Value
	}
	if searchQuery.AfterTurnCount.Present {
		afterTurnCountDB = int32(searchQuery.AfterRating.Value)
	}
	if searchQuery.AfterRating.Present {
		afterRatingDB = searchQuery.AfterRating.Value
	}

	params := query.SelectReplaysByQueryParams{
		PerPage: searchQuery.PerPage,

		// search constraints with mixed 'OR' 'AND' constraints
		UserID:       database.MapOptInt8(searchQuery.UserID),
		WhiteID:      database.MapOptInt8(searchQuery.WhiteID),
		BlackID:      database.MapOptInt8(searchQuery.BlackID),
		WinnerID:     database.MapOptInt8(searchQuery.WinnerID),
		LoserID:      database.MapOptInt8(searchQuery.LoserID),
		Mode:         database.MapOptMode(searchQuery.Mode),
		Result:       database.MapOptResult(searchQuery.Result),
		Cause:        database.MapOptCause(searchQuery.Cause),
		FromDateDays: database.MapOptInt4(toDayEpoch(searchQuery.FromDate)),
		ToDateDays:   database.MapOptInt4(toDayEpoch(searchQuery.ToDate)),

		// search cursor used for pagination, afterID is always provided on a cursor search, rating and turnCount are only provided with sort
		AfterID:        afterIDDB,
		AfterRating:    pgtype.Float8{Float64: afterRatingDB, Valid: true},
		AfterTurnCount: afterTurnCountDB,

		// sort determines the 'ORDER BY' in the SQL query
		SortKey: searchQuery.Sort.String(),
	}
	replayRows, err := service.Querier.SelectReplaysByQuery(ctx, params)
	if err != nil {
		return nil, serrors.New("select replays by query", err, "replaysQuery", searchQuery)
	}

	replays := make([]model.FullReplay, 0, len(replayRows))
	for _, row := range replayRows {
		replays = append(replays, mapFullReplayByIDRow(query.SelectReplayByIDRow(row)))
	}

	slog.InfoContext(ctx, "selected replays", "replaysQuery", searchQuery, "replays", replays)
	return replays, nil
}

type IDByNameRequest struct {
	UsernameInput opt.Option[string] // if not provided, the search query will be ignored
	UserIDOutput  *opt.Option[int64] // if provided, this output is ignored
}

var errUserNotFound = errors.New("user not found")

func (service *ReplaySearchService) loadUserIDsByUsernames(ctx context.Context, searchQuery *ReplaysQuery) error {
	byNameRequests := []IDByNameRequest{
		{UsernameInput: searchQuery.WhiteName, UserIDOutput: &searchQuery.WhiteID},
		{UsernameInput: searchQuery.BlackName, UserIDOutput: &searchQuery.BlackID},
		{UsernameInput: searchQuery.LoserName, UserIDOutput: &searchQuery.LoserID},
		{UsernameInput: searchQuery.WinnerName, UserIDOutput: &searchQuery.WinnerID},
	}

	var usernames []string
	for _, request := range byNameRequests {
		if !request.UsernameInput.Present || request.UserIDOutput.Present {
			continue
		}
		usernames = append(usernames, request.UsernameInput.Value)
	}
	if len(usernames) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "selecting user ids by usernames for requests", "byNameRequests", byNameRequests)

	userRows, err := service.Querier.SelectUserIDsByNames(ctx, usernames)
	if err != nil {
		return serrors.New("select user ids by names", err, "usernames", usernames)
	}

	userIDs := make(map[string]int64)
	for _, row := range userRows {
		userIDs[row.Username] = row.ID
	}

	for _, request := range byNameRequests {
		if !request.UsernameInput.Present {
			continue
		}
		userID, exists := userIDs[request.UsernameInput.Value]
		if !exists {
			return errUserNotFound
		}
		*request.UserIDOutput = opt.Some(userID)
	}
	return nil
}

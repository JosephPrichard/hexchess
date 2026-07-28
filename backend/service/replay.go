package svc

import (
	"context"
	"errors"
	"hexchess-svc/db"
	"hexchess-svc/db/query"

	"hexchess-svc/model"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/optional"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrNoReplay = errors.New("replay not found")

func (services *HexchessServices) GetReplayByGameID(ctx context.Context, gameID string) (model.FullReplay, error) {
	row, err := services.readQuerier.SelectReplayByGameID(ctx, gameID)
	return mapGetReplayResult(ctx, gameID, query.SelectReplayByIDRow(row), err)
}

func (services *HexchessServices) GetReplay(ctx context.Context, replayID int64) (model.FullReplay, error) {
	row, err := services.readQuerier.SelectReplayByID(ctx, replayID)
	return mapGetReplayResult(ctx, replayID, row, err)
}

func mapGetReplayResult[ID any](ctx context.Context, id ID, row query.SelectReplayByIDRow, err error) (model.FullReplay, error) {
	if db.IsErrNoRows(err) {
		return model.FullReplay{}, ErrNoReplay
	} else if err != nil {
		return model.FullReplay{}, serrors.New("select replay by id", err, "id", id)
	}
	replay := mapFullReplayByIDRow(row)
	slog.InfoContext(ctx, "selected replay by userID", "replay", replay, "userID", id)
	return replay, nil
}

func mapFullReplayByIDRow(row query.SelectReplayByIDRow) model.FullReplay {
	replay := mapReplayByIDRow(row)
	return model.FullReplay{
		Replay: replay,
		ReplayUsers: model.ReplayUsers{
			WhiteName:    row.WhiteName.String,
			BlackName:    row.BlackName.String,
			WhiteCountry: row.WhiteCountry.String,
			BlackCountry: row.BlackCountry.String,
			WhiteElo:     model.DefaultUserElo(row.WhiteElo),
			BlackElo:     model.DefaultUserElo(row.BlackElo),
		},
		ReplayColorElos: model.NewReplayView(replay),
	}
}

func mapReplayByIDRow(row query.SelectReplayByIDRow) model.Replay {
	replayResult := enum.Expect(row.Result, model.ReplayResultEnums)
	replayCause := enum.Expect(row.Cause, model.ReplayCauseEnums)
	gameMode := enum.Expect(row.Mode, model.GameModeEnums)

	return model.Replay{
		ID:          row.ID,
		WhiteID:     row.WhiteID.Int64,
		BlackID:     row.BlackID.Int64,
		Result:      replayResult,
		Cause:       replayCause,
		Mode:        gameMode,
		WinEloDiff:  row.WinEloDiff,
		LoseEloDiff: row.LoseEloDiff,
		PlayedOn:    row.PlayedOn.Time,
		Rating:      row.Rating.Float64,
		TurnCount:   int(row.TurnCount),
	}
}

func (services *HexchessServices) GetMovesHistory(ctx context.Context, replayID int) ([]byte, error) {
	row, err := services.readQuerier.SelectReplayMoveHistoryByID(ctx, int64(replayID))
	if err != nil {
		return nil, serrors.New("select replay move histories", err, "replayID", replayID)
	}
	slog.InfoContext(ctx, "selected replay move histories", "replayID", replayID)
	return row.Data, nil
}

type ReplaysQuery struct {
	WhiteName  optional.Maybe[string]
	BlackName  optional.Maybe[string]
	WinnerName optional.Maybe[string]
	LoserName  optional.Maybe[string]

	UserID   optional.Maybe[int64]
	WhiteID  optional.Maybe[int64]
	BlackID  optional.Maybe[int64]
	LoserID  optional.Maybe[int64]
	WinnerID optional.Maybe[int64]

	Result   optional.Maybe[model.ReplayResult]
	Mode     optional.Maybe[model.GameMode]
	Cause    optional.Maybe[model.ReplayCause]
	FromDate optional.Maybe[time.Time]
	ToDate   optional.Maybe[time.Time]

	AfterID        optional.Maybe[int64]
	AfterRating    optional.Maybe[float64]
	AfterTurnCount optional.Maybe[int32]

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

func supplyUserID(id *optional.Maybe[int64]) func(int64) {
	return func(userID int64) {
		if !id.Present {
			*id = optional.Just(userID)
		}
	}
}

func (services *HexchessServices) SearchReplaysByQuery(ctx context.Context, q ReplaysQuery) ([]model.FullReplay, error) {
	// get any userIDs requested through the username queries
	err := services.getUserIDsByUsernames(ctx, []UserIDByNameRequest{
		{Username: q.WhiteName, SupplyID: supplyUserID(&q.WhiteID)},
		{Username: q.BlackName, SupplyID: supplyUserID(&q.BlackID)},
		{Username: q.LoserName, SupplyID: supplyUserID(&q.LoserID)},
		{Username: q.WinnerName, SupplyID: supplyUserID(&q.WinnerID)},
	})
	if errors.Is(err, ErrUserNotFound) {
		// if any username cannot be matched to an id, the search query will never yield any replays
		return []model.FullReplay{}, nil
	} else if err != nil {
		return nil, err
	}

	afterID := q.AfterID.OrElse(math.MaxInt64)
	afterTurnCount := q.AfterTurnCount.OrElse(math.MaxInt32)
	afterRating := q.AfterRating.OrElse(math.MaxFloat64)

	// uses the unix epoch in days for range queries on date. this truncates away timestamp precision regarding hours, seconds, etc.
	fromDateDays := optional.Maybe[int32]{Value: timeutil.ToDayEpoch(q.FromDate.Value), Present: q.FromDate.Present}
	toDateDays := optional.Maybe[int32]{Value: timeutil.ToDayEpoch(q.ToDate.Value), Present: q.ToDate.Present}

	params := query.SelectReplaysByQueryParams{
		PerPage: q.PerPage,

		// search constraints with mixed 'OR' 'AND' constraints
		UserID:       db.MapOptInt8(q.UserID),
		WhiteID:      db.MapOptInt8(q.WhiteID),
		BlackID:      db.MapOptInt8(q.BlackID),
		WinnerID:     db.MapOptInt8(q.WinnerID),
		LoserID:      db.MapOptInt8(q.LoserID),
		Mode:         db.MapOptMode(q.Mode),
		Result:       db.MapOptResult(q.Result),
		Cause:        db.MapOptCause(q.Cause),
		FromDateDays: db.MapOptInt4(fromDateDays),
		ToDateDays:   db.MapOptInt4(toDateDays),

		// search cursor used for pagination, afterID is always provided on a cursor search, rating and turnCount are only provided with sort
		AfterID:        afterID,
		AfterRating:    pgtype.Float8{Float64: afterRating, Valid: true},
		AfterTurnCount: afterTurnCount,

		// sort determines the 'ORDER BY' in the SQL query
		SortKey: q.Sort.String(),
	}
	replayRows, err := services.readQuerier.SelectReplaysByQuery(ctx, params)
	if err != nil {
		return nil, serrors.New("select replays by query", err)
	}

	replays := make([]model.FullReplay, 0, len(replayRows))
	for _, row := range replayRows {
		replays = append(replays, mapFullReplayByIDRow(query.SelectReplayByIDRow(row)))
	}

	slog.InfoContext(ctx, "selected replays", "replaysQuery", q, "replays", replays)
	return replays, nil
}

const (
	ShortBucketDuration = 24 * time.Hour
	LongBucketDuration  = 7 * 24 * time.Hour
)

type EloHistoriesParams struct {
	UserID    int64     `json:"userID"`
	Months    uint      `json:"months"`
	TimeUntil time.Time `json:"timeUntil"`
}

type EloHistoryBuckets map[string][]EloHistoryBucket

type EloHistoryBucket struct {
	Timestamp string  `json:"timestamp"`
	Elo       float64 `json:"elo"`
}

type RetrieveEloHistoryResp struct {
	EloHistories   EloHistoryBuckets
	BucketDuration time.Duration
}

// RetrieveEloHistoryBuckets Returns the elo replay histories for a given user organized into buckets and categorized into a map keyed by replay "mode"
// map will contain the keys "ALL" (contains payload for all modes) plus all modes (ReplayModes)
func (services *HexchessServices) RetrieveEloHistoryBuckets(ctx context.Context, params EloHistoriesParams) (RetrieveEloHistoryResp, error) {
	if params.TimeUntil.IsZero() {
		params.TimeUntil = time.Now()
	}
	playedAfter := pgtype.Timestamptz{}
	if params.Months != 0 {
		playedAfter = pgtype.Timestamptz{Valid: true, Time: params.TimeUntil.AddDate(0, -int(params.Months), 0)}
	}

	eloRows, err := services.readQuerier.SelectReplayElos(ctx, query.SelectReplayElosParams{
		ID:          pgtype.Int8{Int64: params.UserID, Valid: true},
		PlayedAfter: playedAfter,
	})
	if err != nil {
		return RetrieveEloHistoryResp{}, serrors.New("select replay elos for user", err, "userID", params.UserID, "playedAfter", playedAfter)
	}
	slog.InfoContext(ctx, "selected elo replay histories", "userID", params.UserID, "playedAfter", playedAfter, "eloRows", eloRows)

	buckets, durations := aggregateEloHistoryBuckets(eloRows, params)

	slog.InfoContext(ctx, "retrieved elo histories", "userID", params.UserID, "eloHistoryBuckets", buckets)
	return RetrieveEloHistoryResp{EloHistories: buckets, BucketDuration: durations}, nil
}

const year = 365 * 24 * time.Hour

type Bucket struct {
	eloTotal  float64 // accumulating average.
	eloCount  int
	startTime time.Time          // begin the time range of the Bucket we are accumulating.
	elements  []EloHistoryBucket // all accumulated buckets.
}

type BucketMap map[model.GameMode]*Bucket

func (buckets BucketMap) get(mode model.GameMode) *Bucket {
	bucket, ok := buckets[mode]
	if !ok {
		bucket = &Bucket{}
		buckets[mode] = bucket
	}
	return bucket
}

// a bucket contains the averaged payload over a certain timeframe.
func appendBucket(bucket *Bucket, duration time.Duration) {
	if bucket.eloCount <= 0 || bucket.startTime.IsZero() {
		return
	}
	t := bucket.startTime.Truncate(duration)
	bucket.elements = append(bucket.elements, EloHistoryBucket{
		Elo:       bucket.eloTotal / float64(bucket.eloCount),
		Timestamp: t.Format(time.RFC3339),
	})
}

func aggregateEloHistoryBuckets(eloRows []query.SelectReplayElosRow, params EloHistoriesParams) (EloHistoryBuckets, time.Duration) {
	bucketMap := make(EloHistoryBuckets)

	if len(eloRows) == 0 {
		return bucketMap, 0
	}

	firstTime := eloRows[0].PlayedOn.Time
	lastTime := eloRows[len(eloRows)-1].PlayedOn.Time

	isLongHistory := lastTime.Sub(firstTime) > year
	duration := ShortBucketDuration
	if isLongHistory {
		duration = LongBucketDuration
	}

	buckets := BucketMap{}

	// fill buckets row by row, a Bucket is filled once the startTime is 'duration' ago relative to the current row
	for _, row := range eloRows {
		mode, ok := model.GameModeEnums[string(row.Mode)]
		if !ok {
			continue
		}
		bucket := buckets.get(mode)

		var elo float64
		switch params.UserID {
		case row.WhiteID.Int64:
			elo = row.WhiteElo
		case row.BlackID.Int64:
			elo = row.BlackElo
		default:
			continue
		}

		bucketEnd := bucket.startTime.Add(duration)
		isFilledBucket := row.PlayedOn.Time.After(bucketEnd)

		if bucket.startTime.IsZero() {
			bucket.eloTotal += elo
			bucket.eloCount++
			bucket.startTime = row.PlayedOn.Time
		} else if isFilledBucket {
			appendBucket(bucket, duration)
			bucket.eloTotal = elo
			bucket.eloCount = 1
			bucket.startTime = row.PlayedOn.Time
		} else {
			bucket.eloTotal += elo
			bucket.eloCount++
		}
	}
	// append any buckets that may not have been fully filled, but contain averaged payload.
	for _, mode := range model.GameModeEnums {
		appendBucket(buckets.get(mode), duration)
	}

	for mode, acc := range buckets {
		if len(acc.elements) > 0 {
			bucketMap[mode.String()] = acc.elements
		}
	}
	return bucketMap, duration
}

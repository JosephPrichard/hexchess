package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/model"
	"hexchess-svc/util/enum"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrNoReplay = errors.New("replay not found")

func (svc *HexchessServices) GetReplayByGameID(ctx context.Context, gameID string) (model.FullReplay, error) {
	row, err := svc.querier.SelectReplayByGameID(ctx, gameID)
	return mapGetReplayResult(ctx, gameID, sqlc.SelectReplayByIDRow(row), err)
}

func (svc *HexchessServices) GetReplay(ctx context.Context, replayID int64) (model.FullReplay, error) {
	row, err := svc.querier.SelectReplayByID(ctx, replayID)
	return mapGetReplayResult(ctx, replayID, row, err)
}

func mapGetReplayResult[ID any](ctx context.Context, id ID, row sqlc.SelectReplayByIDRow, err error) (model.FullReplay, error) {
	if IsErrNoRows(err) {
		return model.FullReplay{}, ErrNoReplay
	} else if err != nil {
		return model.FullReplay{}, fmt.Errorf("select replay [%v] by existingID: %w", id, err)
	}
	replay := mapFullReplayByIDRow(row)
	slog.InfoContext(ctx, "selected replay by existingID", "replay", replay, "existingID", id)
	return replay, nil
}

func mapFullReplayByIDRow(row sqlc.SelectReplayByIDRow) model.FullReplay {
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
		RepayView: model.MakeReplayView(replay),
	}
}

func mapReplayByIDRow(row sqlc.SelectReplayByIDRow) model.Replay {
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
		Rating:      row.Rating,
		TurnCount:   int(row.TurnCount),
	}
}

func (svc *HexchessServices) GetMovesHistory(ctx context.Context, replayID int) ([]byte, error) {
	row, err := svc.querier.SelectReplayMoveHistoryByID(ctx, int64(replayID))
	if err != nil {
		return nil, fmt.Errorf("select replay move histories for replay [%d]: %w", replayID, err)
	}
	slog.InfoContext(ctx, "selected replay move histories", "replayID", replayID)
	return row.Data, nil
}

type ReplayQuery struct {
	UserID     enum.Optional[int64]
	WhiteID    enum.Optional[int64]
	WhiteName  enum.Optional[string]
	BlackID    enum.Optional[int64]
	BlackName  enum.Optional[string]
	LoserID    enum.Optional[int64]
	LoserName  enum.Optional[string]
	WinnerID   enum.Optional[int64]
	WinnerName enum.Optional[string]
	Result     enum.Optional[model.ReplayResult]
	Mode       enum.Optional[model.GameMode]
	Cause      enum.Optional[model.ReplayCause]
	FromDate   enum.Optional[time.Time]
	ToDate     enum.Optional[time.Time]

	AfterID        enum.Optional[int64]
	AfterRating    enum.Optional[float64]
	AfterTurnCount enum.Optional[int64]

	Sort ReplayQuerySortKey
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

func (svc *HexchessServices) joinReplayQueryUsers(ctx context.Context, replayQuery *ReplayQuery) error {
	var usernames []string

	for _, username := range []enum.Optional[string]{
		replayQuery.WhiteName,
		replayQuery.BlackName,
		replayQuery.WinnerName,
		replayQuery.LoserName,
	} {
		if username.IsPresent {
			usernames = append(usernames, username.Value)
		}
	}

	if len(usernames) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "selecting user ids by usernames for replay query", "usernames", usernames, "replayQuery", replayQuery)

	userRows, err := svc.querier.SelectUserIDsByNames(ctx, usernames)
	if err != nil {
		return fmt.Errorf("select user ids by names %v: %w", usernames, err)
	}

	if len(userRows) != len(usernames) {
		return ErrUsernameNotFound
	}

	userIDs := make(map[string]int64)
	for _, row := range userRows {
		userIDs[row.Username] = row.ID
	}

	for _, pair := range []struct {
		existingID       *enum.Optional[int64]
		incomingUsername enum.Optional[string]
	}{
		{&replayQuery.WhiteID, replayQuery.WhiteName},
		{&replayQuery.BlackID, replayQuery.BlackName},
		{&replayQuery.WinnerID, replayQuery.WinnerName},
		{&replayQuery.LoserID, replayQuery.LoserName},
	} {
		if pair.incomingUsername.IsPresent && !pair.existingID.IsPresent {
			*pair.existingID = enum.OptionalOf(userIDs[pair.incomingUsername.Value])
		}
	}

	return nil
}

func (svc *HexchessServices) SearchReplaysByQuery(ctx context.Context, query ReplayQuery, perPage int32) ([]model.FullReplay, error) {
	if err := svc.joinReplayQueryUsers(ctx, &query); err != nil {
		if err == ErrUsernameNotFound {
			// if an ID could not be loaded for a user that is provided as part of the search query, no replays will ever meet this search criterea.
			return []model.FullReplay{}, nil
		}
		return nil, err
	}

	afterID := int64(math.MaxInt64)
	if query.AfterID.IsPresent {
		afterID = query.AfterID.Value
	}
	afterTurnCount := int32(math.MaxInt32)
	if query.AfterTurnCount.IsPresent {
		afterTurnCount = int32(query.AfterTurnCount.Value)
	}
	afterRating := math.MaxFloat64
	if query.AfterRating.IsPresent {
		afterRating = query.AfterRating.Value
	}

	modeStr := query.Mode.Value.String()
	resultStr := query.Result.Value.String()
	causeStr := query.Cause.Value.String()

	params := sqlc.SelectReplaysByQueryParams{
		// Limit is parameterized but not user input
		PerPage: perPage,

		// Search constraints with 'AND' constraints
		UserID:   pgtype.Int8{Int64: query.UserID.Value, Valid: query.UserID.IsPresent},
		WinnerID: pgtype.Int8{Int64: query.WinnerID.Value, Valid: query.WinnerID.IsPresent},
		LoserID:  pgtype.Int8{Int64: query.LoserID.Value, Valid: query.LoserID.IsPresent},
		WhiteID:  pgtype.Int8{Int64: query.WhiteID.Value, Valid: query.WhiteID.IsPresent},
		BlackID:  pgtype.Int8{Int64: query.BlackID.Value, Valid: query.BlackID.IsPresent},
		Mode:     sqlc.NullModeEnum{ModeEnum: sqlc.ModeEnum(modeStr), Valid: query.Mode.IsPresent},
		Result:   sqlc.NullResultEnum{ResultEnum: sqlc.ResultEnum(resultStr), Valid: query.Result.IsPresent},
		Cause:    sqlc.NullCauseEnum{CauseEnum: sqlc.CauseEnum(causeStr), Valid: query.Cause.IsPresent},
		DateFrom: pgtype.Timestamptz{Time: query.FromDate.Value, Valid: query.FromDate.IsPresent},
		DateTo:   pgtype.Timestamptz{Time: query.ToDate.Value, Valid: query.ToDate.IsPresent},

		// Search cursor used for pagination, afterID is always provided on a cursor search, rating and turnCount are only provided with sort
		AfterID:        afterID,
		AfterRating:    afterRating,
		AfterTurnCount: afterTurnCount,

		// Sort determines the 'ORDER BY' in the SQL query
		SortKey: query.Sort.String(),
	}
	replayRows, err := svc.querier.SelectReplaysByQuery(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("select replays by query %+v: %w", query, err)
	}

	replays := make([]model.FullReplay, 0, len(replayRows))
	for _, row := range replayRows {
		replays = append(replays, mapFullReplayByIDRow(sqlc.SelectReplayByIDRow(row)))
	}

	slog.InfoContext(ctx, "selected replays", "query", query, "perPage", perPage, "replays", replays)
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

// RetrieveEloHistoryBuckets Returns the elo replay histories for a given user organized into buckets and categorized into a map keyed by replay "mode"
// map will contain the keys "ALL" (contains payload for all modes) plus all modes (ReplayModes)
func (svc *HexchessServices) RetrieveEloHistoryBuckets(ctx context.Context, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
	if params.TimeUntil.IsZero() {
		params.TimeUntil = time.Now()
	}
	playedAfter := pgtype.Timestamptz{}
	if params.Months != 0 {
		playedAfter = pgtype.Timestamptz{Valid: true, Time: params.TimeUntil.AddDate(0, -int(params.Months), 0)}
	}

	eloRows, err := svc.querier.SelectReplayElos(ctx, sqlc.SelectReplayElosParams{
		ID:          pgtype.Int8{Int64: params.UserID, Valid: true},
		PlayedAfter: playedAfter,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("select replay elos for user %d after %v: %w", params.UserID, playedAfter, err)
	}
	slog.InfoContext(ctx, "selected elo replay histories", "userID", params.UserID, "playedAfter", playedAfter, "eloRows", eloRows)

	buckets, durations := aggregateEloHistoryBuckets(eloRows, params)

	slog.InfoContext(ctx, "retrieved elo histories", "userID", params.UserID, "eloHistoryBuckets", buckets)
	return buckets, durations, nil
}

const year = 365 * 24 * time.Hour

type Bucket struct {
	eloTotal  float64 // accumulating average.
	eloCount  int
	startTime time.Time          // begin time range of the Bucket we are accumulating.
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

func aggregateEloHistoryBuckets(eloRows []sqlc.SelectReplayElosRow, params EloHistoriesParams) (EloHistoryBuckets, time.Duration) {
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

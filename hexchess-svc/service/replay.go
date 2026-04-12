package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/util/enum"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ReplayDTO struct {
	ID          int64        `json:"id"`
	WhiteID     int64        `json:"whiteId"`
	BlackID     int64        `json:"blackId"`
	Mode        GameMode     `json:"mode"`
	Result      ReplayResult `json:"result"`
	Cause       ReplayCause  `json:"cause"`
	WinEloDiff  float64      `json:"winEloDiff"`
	LoseEloDiff float64      `json:"loseEloDiff"`
	PlayedOn    time.Time    `json:"playedOn"`
}

type ReplayUsersDTO struct {
	WhiteName    string  `json:"whiteName"`
	BlackName    string  `json:"blackName"`
	WhiteCountry string  `json:"whiteCountry"`
	BlackCountry string  `json:"blackCountry"`
	WhiteElo     float64 `json:"whiteElo"`
	BlackElo     float64 `json:"blackElo"`
}

type ReplayViewDTO struct {
	WhiteEloDiff float64 `json:"whiteEloDiff"`
	BlackEloDiff float64 `json:"blackEloDiff"`
}

type FullReplayDTO struct {
	ReplayDTO
	ReplayUsersDTO
	ReplayViewDTO
}

func MakeReplayViewDTO(input ReplayDTO) (output ReplayViewDTO) {
	switch input.Result {
	case WhiteWin:
		output.WhiteEloDiff, output.BlackEloDiff = input.WinEloDiff, input.LoseEloDiff
	case BlackWin:
		output.WhiteEloDiff, output.BlackEloDiff = input.LoseEloDiff, input.WinEloDiff
	default:
	}
	return
}

var ErrNoReplay = errors.New("replay not found")

func (svc *HexchessServices) GetReplayByGameID(ctx context.Context, gameID string) (FullReplayDTO, error) {
	row, err := svc.querier.SelectReplayByGameID(ctx, gameID)
	return mapGetReplayResult(ctx, gameID, sqlc.SelectReplayByIDRow(row), err)
}

func (svc *HexchessServices) GetReplay(ctx context.Context, replayID int64) (FullReplayDTO, error) {
	row, err := svc.querier.SelectReplayByID(ctx, replayID)
	return mapGetReplayResult(ctx, replayID, row, err)
}

func mapGetReplayResult[ID any](ctx context.Context, id ID, row sqlc.SelectReplayByIDRow, err error) (FullReplayDTO, error) {
	if IsErrNoRows(err) {
		return FullReplayDTO{}, ErrNoReplay
	} else if err != nil {
		return FullReplayDTO{}, fmt.Errorf("select replay [%v] by id: %w", id, err)
	}
	replay := mapFullReplayByIDRow(row)
	slog.InfoContext(ctx, "selected replay by id", "replay", replay, "id", id)
	return replay, nil
}

func mapFullReplayByIDRow(row sqlc.SelectReplayByIDRow) FullReplayDTO {
	replay := mapReplayByIDRow(row)
	return FullReplayDTO{
		ReplayDTO: replay,
		ReplayUsersDTO: ReplayUsersDTO{
			WhiteName:    row.WhiteName.String,
			BlackName:    row.BlackName.String,
			WhiteCountry: row.WhiteCountry.String,
			BlackCountry: row.BlackCountry.String,
			WhiteElo:     defaultElo(row.WhiteElo),
			BlackElo:     defaultElo(row.BlackElo),
		},
		ReplayViewDTO: MakeReplayViewDTO(replay),
	}
}

func mapReplayByIDRow(row sqlc.SelectReplayByIDRow) ReplayDTO {
	replayResult := enum.Expect(row.Result, ReplayResultEnums)
	replayCause := enum.Expect(row.Cause, ReplayCauseEnums)
	gameMode := enum.Expect(row.Mode, GameModeEnums)

	return ReplayDTO{
		ID:          row.ID,
		WhiteID:     row.WhiteID.Int64,
		BlackID:     row.BlackID.Int64,
		Result:      replayResult,
		Cause:       replayCause,
		Mode:        gameMode,
		WinEloDiff:  row.WinEloDiff,
		LoseEloDiff: row.LoseEloDiff,
		PlayedOn:    row.PlayedOn.Time,
	}
}

func (svc *HexchessServices) GetMovesHistory(ctx context.Context, replayID int) ([]byte, error) {
	row, err := svc.querier.SelectReplayMoveHistories(ctx, int64(replayID))
	if err != nil {
		return nil, fmt.Errorf("select replay move histories for replay [%d]: %w", replayID, err)
	}
	slog.InfoContext(ctx, "selected replay move histories", "replayID", replayID)
	return row.Data, nil
}

func (svc *HexchessServices) GetUserReplays(ctx context.Context, userID int64, afterID int64, perPage int32) ([]FullReplayDTO, error) {
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	replayRows, err := svc.querier.SelectUserReplays(ctx, sqlc.SelectUserReplaysParams{
		UserID:  pgtype.Int8{Int64: userID, Valid: true},
		AfterID: afterID,
		PerPage: perPage,
	})
	if err != nil {
		return nil, fmt.Errorf("select replays by user id [%d]: %w", userID, err)
	}

	replays := make([]FullReplayDTO, 0, len(replayRows))
	for _, row := range replayRows {
		replays = append(replays, mapFullReplayByIDRow(sqlc.SelectReplayByIDRow(row)))
	}

	slog.InfoContext(ctx, "selected replays", "replays", replays, "userID", userID, "afterID", afterID, "perPage", perPage)
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
// map will contain the keys "ALL" (contains data for all modes) plus all modes (ReplayModes)
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

type BucketMap map[GameMode]*Bucket

func (buckets BucketMap) get(mode GameMode) *Bucket {
	bucket, ok := buckets[mode]
	if !ok {
		bucket = &Bucket{}
		buckets[mode] = bucket
	}
	return bucket
}

// a bucket contains the averaged data over a certain timeframe.
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
		mode, ok := GameModeEnums[string(row.Mode)]
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
	// append any buckets that may not have been fully filled, but contain averaged data.
	for _, mode := range GameModeEnums {
		appendBucket(buckets.get(mode), duration)
	}

	for mode, acc := range buckets {
		if len(acc.elements) > 0 {
			bucketMap[mode.String()] = acc.elements
		}
	}
	return bucketMap, duration
}

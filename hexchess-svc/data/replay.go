package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"
)

type ReplayEntity struct {
	ID           int64        `json:"id"`
	WhiteID      int64        `json:"whiteId"`
	BlackID      int64        `json:"blackId"`
	WhiteName    string       `json:"whiteName"`
	BlackName    string       `json:"blackName"`
	WhiteCountry string       `json:"whiteCountry"`
	BlackCountry string       `json:"blackCountry"`
	Result       ReplayResult `json:"result"`
	Cause        ReplayCause  `json:"cause"`
	WinElo       float64      `json:"winElo"`
	LoseElo      float64      `json:"loseElo"`
	WhiteElo     float64      `json:"whiteElo"`
	BlackElo     float64      `json:"blackElo"`
	WhiteEloDiff float64      `json:"whiteEloDiff"`
	BlackEloDiff float64      `json:"blackEloDiff"`
	PlayedOn     time.Time    `json:"playedOn"`
}

var ReplayEntityCmpOpts = cmpopts.IgnoreFields(ReplayEntity{}, "PlayedOn")

type ReplayResult string

const (
	WhiteWin ReplayResult = "WHITE_WINS"
	BlackWin ReplayResult = "BLACK_WINS"
	Draw     ReplayResult = "DRAW"
)

type ReplayCause string

const (
	Checkmate ReplayCause = "CHECKMATE"
	Forfeit   ReplayCause = "FORFEIT"
)

type ReplayInst struct {
	WhiteID          int64        `json:"whiteId"`
	BlackID          int64        `json:"blackId"`
	Result           ReplayResult `json:"result"`
	Cause            ReplayCause  `json:"cause"`
	WinElo           float64      `json:"winElo"`
	LoseElo          float64      `json:"loseElo"`
	MoveHistoryProto []byte
}

func InsertReplay(ctx context.Context, query *db.Queries, inst ReplayInst) (int64, error) {
	if inst.MoveHistoryProto == nil {
		inst.MoveHistoryProto = []byte{}
	}

	replayID, err := query.InsertReplay(ctx, db.InsertReplayParams{
		WhiteID:     inst.WhiteID,
		BlackID:     inst.BlackID,
		Result:      string(inst.Result),
		Cause:       string(inst.Cause),
		WinElo:      inst.WinElo,
		LoseElo:     inst.LoseElo,
		MoveHistory: inst.MoveHistoryProto,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to create replay", "replay", inst, "err", err)
		return 0, err
	}

	inst.MoveHistoryProto = nil
	slog.InfoContext(ctx, "created a new replay", "replay", inst, "replayID", replayID)
	return replayID, nil
}

func colorEloDiffs(result ReplayResult, winElo float64, loseElo float64) (float64, float64) {
	var whiteEloDiff, blackEloDiff float64
	switch result {
	case WhiteWin:
		whiteEloDiff, blackEloDiff = winElo, loseElo
	case BlackWin:
		whiteEloDiff, blackEloDiff = loseElo, winElo
	default:
	}
	return whiteEloDiff, blackEloDiff
}

func mapReplayFromRow(row db.GetReplayByIDRow) (ReplayEntity, error) {
	replay := ReplayEntity{
		ID:           row.ID,
		WhiteID:      row.WhiteID,
		BlackID:      row.BlackID,
		WhiteName:    row.WhiteName,
		BlackName:    row.BlackName,
		WhiteCountry: row.WhiteCountry.String,
		BlackCountry: row.BlackCountry.String,
		Result:       ReplayResult(row.Result),
		Cause:        ReplayCause(row.Cause),
		WinElo:       row.WinElo,
		LoseElo:      row.LoseElo,
		WhiteElo:     row.WhiteElo,
		BlackElo:     row.BlackElo,
		PlayedOn:     row.PlayedOn.Time,
	}
	replay.WhiteEloDiff, replay.BlackEloDiff = colorEloDiffs(replay.Result, replay.WinElo, replay.LoseElo)
	return replay, nil
}

var ErrNoReplay = errors.New("replay not found")

func GetReplay(ctx context.Context, query *db.Queries, id int64) (ReplayEntity, error) {
	row, err := query.GetReplayByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ReplayEntity{}, ErrNoReplay
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to select replay", "id", id, "err", err)
		return ReplayEntity{}, fmt.Errorf("failed to get replay %d by id: %w", id, err)
	}
	replay, err := mapReplayFromRow(row)
	if err != nil {
		return ReplayEntity{}, err
	}

	slog.InfoContext(ctx, "selected replay by id", "replay", replay)
	return replay, nil
}

func GetReplayMoveHistory(ctx context.Context, query *db.Queries, id int64) (*pb.MoveHistory, error) {
	b, err := query.GetReplayMoveHistory(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoReplay
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get replay %d move list by id: %w", id, err)
	}

	var moveHist pb.MoveHistory
	if err := proto.Unmarshal(b, &moveHist); err != nil {
		return nil, fmt.Errorf("failed to unmarshal move history: %w", err)
	}

	return &moveHist, nil
}

func GetUserReplays(ctx context.Context, query *db.Queries, userID int64, afterID int64, perPage int32) ([]ReplayEntity, error) {
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	rows, err := query.GetUserReplays(ctx, db.GetUserReplaysParams{
		UserID:  userID,
		AfterID: afterID,
		PerPage: perPage,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to select replays", "userID", userID, "afterID", afterID, "err", err)
		return nil, fmt.Errorf("failed to select replays: %w", err)
	}

	var replays []ReplayEntity
	for _, row := range rows {
		replay, err := mapReplayFromRow(db.GetReplayByIDRow(row))
		if err != nil {
			return nil, err
		}
		replays = append(replays, replay)
	}
	slog.InfoContext(ctx, "selected replays", "replays", replays, "userID", userID, "afterID", afterID, "perPage", perPage)
	return replays, nil
}

func GetEloHistories(ctx context.Context, rdb *Redis, userID int64) (*pb.EloHistories, error) {
	key := "elo-history:" + strconv.Itoa(int(userID))
	var isCached bool

	conn := rdb.Cache.Get()
	defer conn.Close()

	var pbEloHistories pb.EloHistories

	v, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		if errors.Is(err, redis.ErrNil) {
			isCached = true
		} else {
			return nil, fmt.Errorf("failed to 'GET' cached elo histories for user %d: %w", userID, err)
		}
	}	
	if isCached {
		if err := proto.Unmarshal(v, &pbEloHistories); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached elo histories for user %d: %w", userID, err)
		}
	}

	return &pbEloHistories, nil
}

func SetEloHistories(tx context.Context, rdb *Redis, userID int64, pbHistories *pb.EloHistories) error {
	key := "elo-history:" + strconv.Itoa(int(userID))

	conn := rdb.Cache.Get()
	defer conn.Close()

	v, err := proto.Marshal(pbHistories)
	if err != nil {
		return fmt.Errorf("failed to marshal elo histories for user %d: %w", userID, err)
	}
	if _, err := conn.Do("SET", key, v); err != nil {
		return fmt.Errorf("failed to 'SET' cached elo histories for user %d: %w", userID, err)
	}
	return nil
}

func RetrieveEloHistories(ctx context.Context, pdb *Postgres, rdb *Redis, userID int64) (*pb.EloHistories, error) {
	fail := func(err error) (*pb.EloHistories, error) {
		slog.ErrorContext(ctx, "failed to get elo histories for user", "err", err, "userID", userID)
		return nil, err
	}
	
	pbEloHistories, err := GetEloHistories(ctx, rdb, userID)
	if err != nil {
		return fail(err)
	}

	var lastHistTs pgtype.Timestamptz
	histElo := StartElo

	if len(pbEloHistories.Histories) > 0 {
		lastHist := pbEloHistories.Histories[len(pbEloHistories.Histories)-1]
		if lastHist == nil {
			return fail(errors.New("last elo history was nil, must not be"))
		}
		t, err := time.Parse(time.RFC3339, lastHist.Timestamp)
		if err != nil {
			return fail(fmt.Errorf("failed to parse elo history timestamp %s: %w", lastHist.Timestamp, err))
		}
		lastHistTs = pgtype.Timestamptz{Valid: true, Time: t}
		histElo = lastHist.Elo
	}

	eloRows, err := pdb.Query.GetReplayElos(ctx, db.GetReplayElosParams{
		ID: userID,
		PlayedAfter: lastHistTs,
	})
	if err != nil {
		return fail(fmt.Errorf("failed to select replay elos: %w", err))
	}
	
	beforeHistoriesCnt := len(pbEloHistories.Histories)
	for _, row := range eloRows {
		whiteEloDiff, blackEloDiff := colorEloDiffs(ReplayResult(row.Result), row.WinElo, row.LoseElo)
		var eloDiff float64
		switch userID {
		case row.WhiteID:
			eloDiff = whiteEloDiff
		case row.BlackID:
			eloDiff = blackEloDiff
		default:
			return fail(fmt.Errorf("assertion error: at least one elo hist row %v player IDs must match provided user ID %d", row, userID))
		}
		histElo += eloDiff
		pbEloHistories.Histories = append(pbEloHistories.Histories, &pb.EloHistory{
			Timestamp: row.PlayedOn.Time.Format(time.RFC3339),
			Elo: histElo,
		})
	}

	if len(pbEloHistories.Histories) != beforeHistoriesCnt {
		if err := SetEloHistories(ctx, rdb, userID, pbEloHistories); err != nil {
			return fail(err)
		}
	}

	return pbEloHistories, nil
}

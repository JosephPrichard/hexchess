package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/db"
	"hexchess-svc/logs"
	"log/slog"
	"time"
)

type ColorSelect int

const (
	White ColorSelect = iota
	Black
	Random
)

type TimeControl int

const (
	RealTime TimeControl = iota
	Correspondence
	Unlimited
)

func (tc TimeControl) String() string {
	switch tc {
	case RealTime:
		return "REAL_TIME"
	case Correspondence:
		return "CORRESPONDENCE"
	case Unlimited:
		return "UNLIMITED"
	default:
		return "UNKNOWN"
	}
}

func (cs ColorSelect) String() string {
	switch cs {
	case White:
		return "WHITE"
	case Black:
		return "BLACK"
	case Random:
		return "RANDOM"
	default:
		return "UNKNOWN"
	}
}

type ChallengeEntity struct {
	ChallengerID      int64
	ChallengerName    string
	ChallengerCountry string
	ChallengerElo     float64
	ChallengeeId      int64
	ChallengeeName    string
	ChallengeeCountry string
	ChallengeeElo     float64
	TimeControl       TimeControl
	StartColor        ColorSelect // from challenger's perspective
	MadeOn            time.Time
}

var (
	ErrDuplicateChallenge  = errors.New("duplicate challenge")
	ErrSelfChallenge       = errors.New("cannot challenge yourself")
	ErrParticipantConflict = errors.New("user already in a challenge")
	ErrChallengeNotFound   = errors.New("challenge not found")
)

const ExpireChallengeThreshold = time.Hour * 24 * 7

type ChallengeInst struct {
	ChallengerID int64       `json:"challengerId"`
	ChallengeeID int64       `json:"challengeeId"`
	TimeControl  TimeControl `json:"timeControl"`
	StartColor   ColorSelect `json:"startColor"`
	MadeOn       time.Time   `json:"madeOn"`
}

func mapChallengeFromRow(row db.SelectChallengesByParticipantRow) (ChallengeEntity, error) {
	tc, err := ParseTimeControl(row.TimeControl)
	if err != nil {
		return ChallengeEntity{}, err
	}
	cs, err := ParseColorSelect(row.StartColor)
	if err != nil {
		return ChallengeEntity{}, err
	}
	return ChallengeEntity{
		ChallengerID:      row.ChallengerID,
		ChallengerName:    row.ChallengerName,
		ChallengerCountry: row.ChallengerCountry.String,
		ChallengerElo:     row.ChallengerElo,
		ChallengeeId:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry.String,
		ChallengeeElo:     row.ChallengeeElo,
		TimeControl:       tc,
		StartColor:        cs,
		MadeOn:            row.MadeOn.Time,
	}, nil
}

func ParseTimeControl(tc string) (TimeControl, error) {
	switch tc {
	case "UNLIMITED":
		return Unlimited, nil
	case "REAL_TIME":
		return RealTime, nil
	case "CORRESPONDENCE":
		return Correspondence, nil
	default:
		return 0, fmt.Errorf("unparseable TimeControl value: %s", tc)
	}
}

func ParseColorSelect(cs string) (ColorSelect, error) {
	switch cs {
	case "WHITE":
		return White, nil
	case "BLACK":
		return Black, nil
	case "RANDOM":
		return Random, nil
	default:
		return 0, fmt.Errorf("unparseable ColorSelect value: %s", cs)
	}
}

func InsertChallenge(ctx context.Context, q *db.Queries, inst ChallengeInst) error {
	_, err := InsertChallengeRet(ctx, q, inst)
	return err
}

func InsertChallengeRet(ctx context.Context, q *db.Queries, inst ChallengeInst) (ChallengeEntity, error) {
	if inst.ChallengerID == inst.ChallengeeID {
		return ChallengeEntity{}, ErrSelfChallenge
	}

	if inst.MadeOn.IsZero() {
		inst.MadeOn = time.Now()
	}

	row, err := q.InsertChallenge(ctx, db.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		TimeControl:  inst.TimeControl.String(),
		StartColor:   inst.StartColor.String(),
		MadeOn:       pgtype.Timestamp{Valid: true, Time: inst.MadeOn},
	})

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			slog.InfoContext(ctx, "challenge already exists", "challenge", inst, "err", pgErr)
			return ChallengeEntity{}, ErrDuplicateChallenge
		case "23503", "23506":
			slog.InfoContext(ctx, "violating key constraint when creating challenge", "challenge", inst, "err", pgErr)
			return ChallengeEntity{}, ErrParticipantConflict
		}
	}

	challenge, err := mapChallengeFromRow(db.SelectChallengesByParticipantRow(row))
	if err != nil {
		return ChallengeEntity{}, err
	}

	logs.DynLog(ctx, "created a new challenge", err, "challenge", inst, "challenge", challenge)
	return challenge, err
}

const NoChallengeID = -1

func GetChallengesByParticipant(ctx context.Context, q *db.Queries, challengerID int64, challengeeID int64, threshold time.Duration) ([]ChallengeEntity, error) {
	var pgChallengerID pgtype.Int8
	if challengerID != NoChallengeID {
		pgChallengerID.Valid = true
		pgChallengerID.Int64 = challengerID
	}
	var pgChallengeeID pgtype.Int8
	if challengeeID != NoChallengeID {
		pgChallengeeID.Valid = true
		pgChallengeeID.Int64 = challengeeID
	}

	t := time.Now().Add(-threshold)
	rows, err := q.SelectChallengesByParticipant(ctx, db.SelectChallengesByParticipantParams{
		ChallengerID: pgChallengerID,
		ChallengeeID: pgChallengeeID,
		Since:        pgtype.Timestamp{Valid: true, Time: t},
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to get challenges by participant", "challengerID", challengerID, "challengeeID", challengeeID, "err", err)
		return nil, fmt.Errorf("failed to get challenges by participant: %w", err)
	}

	var challenges []ChallengeEntity
	for _, row := range rows {
		challenge, err := mapChallengeFromRow(row)
		if err != nil {
			return nil, err
		}
		challenges = append(challenges, challenge)
	}

	return challenges, nil
}

type DeleteResult struct {
	ChallengerID int64
	ChallengeeID int64
	TimeControl  TimeControl
	FirstColor   ColorSelect
}

func DeleteChallenge(ctx context.Context, q *db.Queries, challengeeID int64, challengerID int64) (DeleteResult, error) {
	row, err := q.DeleteChallenge(ctx, db.DeleteChallengeParams{ChallengerID: challengeeID, ChallengeeID: challengerID})
	if errors.Is(err, sql.ErrNoRows) {
		return DeleteResult{}, ErrChallengeNotFound
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete challenge", "err", err)
		return DeleteResult{}, err
	}

	tc, err := ParseTimeControl(row.TimeControl)
	if err != nil {
		return DeleteResult{}, err
	}
	cs, err := ParseColorSelect(row.StartColor)
	if err != nil {
		return DeleteResult{}, err
	}
	dr := DeleteResult{ChallengerID: row.ChallengerID, ChallengeeID: row.ChallengeeID, TimeControl: tc, FirstColor: cs}

	slog.InfoContext(ctx, "deleted challenge", "challengee", challengeeID, "challengerID", challengerID, "dr", dr, "err", err)
	return dr, err
}

func DeleteExpiredChallenges(ctx context.Context, q *db.Queries, userID int64, threshold time.Duration) error {
	t := time.Now().Add(-threshold)
	err := q.DeleteExpiredChallenges(ctx, db.DeleteExpiredChallengesParams{
		UserID:     userID,
		ExpireTime: pgtype.Timestamp{Valid: true, Time: t},
	})
	logs.DynLog(ctx, "deleted expired challenges", err, "userID", userID, "expireTime", t, "trace", ctx.Value(logs.TraceKey))
	return nil
}

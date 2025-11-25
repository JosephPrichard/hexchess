package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"log/slog"
	"strings"
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
	ChallengerID      int64       `json:"challengerId"`
	ChallengerName    string      `json:"challengerName"`
	ChallengerCountry string      `json:"challengerCountry"`
	ChallengerElo     float64     `json:"challengerElo"`
	ChallengeeID      int64       `json:"challengeeId"`
	ChallengeeName    string      `json:"challengeeName"`
	ChallengeeCountry string      `json:"challengeeCountry"`
	ChallengeeElo     float64     `json:"challengeeElo"`
	TimeControl       TimeControl `json:"timeControl"`
	StartColor        ColorSelect `json:"startColor"` // from challenger's perspective
	MadeOn            time.Time   `json:"madeOn"`
}

var ChallengeEntityCmpOpts = cmpopts.IgnoreFields(ChallengeEntity{}, "MadeOn")

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
		ChallengeeID:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry.String,
		ChallengeeElo:     row.ChallengeeElo,
		TimeControl:       tc,
		StartColor:        cs,
		MadeOn:            row.MadeOn.Time,
	}, nil
}

func ParseTimeControl(tc string) (TimeControl, error) {
	switch strings.ToUpper(tc) {
	case "UNLIMITED":
		return Unlimited, nil
	case "REAL_TIME":
		return RealTime, nil
	case "CORRESPONDENCE":
		return Correspondence, nil
	default:
		return 0, fmt.Errorf("invalid time control value: %s", tc)
	}
}

func ParseColorSelect(cs string) (ColorSelect, error) {
	switch strings.ToUpper(cs) {
	case "WHITE":
		return White, nil
	case "BLACK":
		return Black, nil
	case "RANDOM":
		return Random, nil
	default:
		return 0, fmt.Errorf("invalid color select value: %s", cs)
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

	row, dbErr := q.InsertChallenge(ctx, db.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		TimeControl:  inst.TimeControl.String(),
		StartColor:   inst.StartColor.String(),
		MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
	})

	var pgErr *pgconn.PgError
	if errors.As(dbErr, &pgErr) {
		switch pgErr.Code {
		case "23505":
			slog.InfoContext(ctx, "challenge already exists", "challenge", inst, "err", pgErr)
			return ChallengeEntity{}, ErrDuplicateChallenge
		case "23503", "23506":
			slog.InfoContext(ctx, "violating key constraint when creating challenge", "challenge", inst, "err", pgErr)
			return ChallengeEntity{}, ErrParticipantConflict
		}
	}
	if dbErr != nil {
		return ChallengeEntity{}, dbErr
	}
	challenge, err := mapChallengeFromRow(db.SelectChallengesByParticipantRow(row))

	util.DynLog(ctx, "created a new challenge", err, "challenge", inst, "challenge", challenge)
	return challenge, err
}

type ChallengeKey struct {
	ChallengerID int64
	ChallengeeID int64
}

func GetChallengesByParticipant(ctx context.Context, q *db.Queries, key ChallengeKey, threshold time.Duration) ([]ChallengeEntity, error) {
	return GetChallengesByParticipantOn(ctx, q, key, time.Now().Add(-threshold))
}

func GetChallengesByParticipantOn(ctx context.Context, q *db.Queries, key ChallengeKey, t time.Time) ([]ChallengeEntity, error) {
	var pgChallengerID pgtype.Int8
	if key.ChallengerID != -1 {
		pgChallengerID.Valid = true
		pgChallengerID.Int64 = key.ChallengerID
	}
	var pgChallengeeID pgtype.Int8
	if key.ChallengeeID != -1 {
		pgChallengeeID.Valid = true
		pgChallengeeID.Int64 = key.ChallengeeID
	}

	rows, err := q.SelectChallengesByParticipant(ctx, db.SelectChallengesByParticipantParams{
		ChallengerID: pgChallengerID,
		ChallengeeID: pgChallengeeID,
		Since:        pgtype.Timestamptz{Valid: true, Time: t},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get challenges by participant %v: %w", key, err)
	}

	var challenges []ChallengeEntity
	for _, row := range rows {
		challenge, err := mapChallengeFromRow(row)
		if err != nil {
			return nil, err
		}
		challenges = append(challenges, challenge)
	}

	slog.InfoContext(ctx, "got challenges by participant", "challengeKey", key, "since", t, "challenges", challenges)
	return challenges, nil
}

type DeleteResult struct {
	ChallengerID int64
	ChallengeeID int64
	TimeControl  TimeControl
	FirstColor   ColorSelect
}

func DeleteChallenge(ctx context.Context, q *db.Queries, key ChallengeKey) (DeleteResult, error) {
	row, err := q.DeleteChallenge(ctx, db.DeleteChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	if errors.Is(err, sql.ErrNoRows) {
		return DeleteResult{}, ErrChallengeNotFound
	}
	if err != nil {
		return DeleteResult{}, fmt.Errorf("failed to delete challenge %d: %w", key, err)
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

	slog.InfoContext(ctx, "deleted challenge", "challengeKey", key, "dr", dr, "err", err)
	return dr, err
}

func DeleteExpiredChallenges(ctx context.Context, q *db.Queries, userID int64, threshold time.Duration) error {
	return DeleteExpiredChallengesOn(ctx, q, userID, time.Now().Add(-threshold))
}

func DeleteExpiredChallengesOn(ctx context.Context, q *db.Queries, userID int64, t time.Time) error {
	err := q.DeleteExpiredChallenges(ctx, db.DeleteExpiredChallengesParams{
		UserID: userID,
		Before: pgtype.Timestamptz{Valid: true, Time: t},
	})
	util.DynLog(ctx, "deleted expired challenges", err, "userID", userID, "expireTime", t)
	return nil
}

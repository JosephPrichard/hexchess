package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"log/slog"
	"time"
)

type ChallengeEntity struct {
	ChallengerID      int64     `json:"challengerId"`
	ChallengerName    string    `json:"challengerName"`
	ChallengerCountry string    `json:"challengerCountry"`
	ChallengerElo     float64   `json:"challengerElo"`
	ChallengeeID      int64     `json:"challengeeId"`
	ChallengeeName    string    `json:"challengeeName"`
	ChallengeeCountry string    `json:"challengeeCountry"`
	ChallengeeElo     float64   `json:"challengeeElo"`
	Mode              GameMode  `json:"mode"`
	StartColor        Color     `json:"startColor"` // from challenger's perspective
	MadeOn            time.Time `json:"madeOn"`
	ExpiresOn         time.Time `json:"expiresOn"`
}

var (
	ErrDuplicateChallenge  = errors.New("duplicate challenge")
	ErrSelfChallenge       = errors.New("cannot challenge yourself")
	ErrParticipantConflict = errors.New("user already in a challenge")
	ErrChallengeNotFound   = errors.New("challenge not found")
)

const ExpireChallengeThreshold = time.Hour * 24 * 7

type ChallengeInst struct {
	ChallengerID int64     `json:"challengerId"`
	ChallengeeID int64     `json:"challengeeId"`
	Mode         GameMode  `json:"mode"`
	StartColor   Color     `json:"startColor"`
	MadeOn       time.Time `json:"madeOn"`
}

func mapChallengeFromRow(row db.SelectChallengesByParticipantRow) (ChallengeEntity, error) {
	var ce ChallengeEntity
	startColor := Colors.Parse(string(row.StartColor))
	if startColor == nil {
		return ce, fmt.Errorf("invalid start color: %s, expected: %v", row.StartColor, Colors.Values())
	}
	mode := Modes.Parse(string(row.Mode))
	if mode == nil {
		return ce, MakeGameModeError(string(row.Mode))
	}
	ce = ChallengeEntity{
		ChallengerID:      row.ChallengerID,
		ChallengerName:    row.ChallengerName,
		ChallengerCountry: row.ChallengerCountry,
		ChallengerElo:     defaultElo(row.ChallengerElo),
		ChallengeeID:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry,
		ChallengeeElo:     defaultElo(row.ChallengeeElo),
		Mode:              *mode,
		StartColor:        *startColor,
		MadeOn:            row.MadeOn.Time,
		ExpiresOn:         row.MadeOn.Time.Add(ExpireChallengeThreshold),
	}
	return ce, nil
}

func InsertChallenge(ctx context.Context, query *db.Queries, inst ChallengeInst) error {
	_, err := InsertChallengeRet(ctx, query, inst)
	return err
}

func InsertChallengeRet(ctx context.Context, query *db.Queries, inst ChallengeInst) (ChallengeEntity, error) {
	if inst.ChallengerID == inst.ChallengeeID {
		return ChallengeEntity{}, ErrSelfChallenge
	}
	if inst.MadeOn.IsZero() {
		inst.MadeOn = time.Now()
	}

	row, dbErr := query.InsertChallenge(ctx, db.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		Mode:         db.ModeEnum(inst.Mode.Value),
		StartColor:   db.ColorEnum(inst.StartColor.Value),
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

	logutil.DynLog(ctx, "created a new challenge", err, "challenge", inst, "challenge", challenge)
	return challenge, err
}

type ChallengeKey struct {
	ChallengerID int64
	ChallengeeID int64
}

func MakeGetChallengesSince(now time.Time) time.Time {
	return now.Add(-ExpireChallengeThreshold)
}

// GetChallengesByParticipant will select challenges by the participant after the 'since' time
func GetChallengesByParticipant(ctx context.Context, query *db.Queries, key ChallengeKey, since time.Time) ([]ChallengeEntity, error) {
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

	rows, err := query.SelectChallengesByParticipant(ctx, db.SelectChallengesByParticipantParams{
		ChallengerID: pgChallengerID,
		ChallengeeID: pgChallengeeID,
		Since:        pgtype.Timestamptz{Valid: true, Time: since},
	})
	if err != nil {
		return nil, fmt.Errorf("get challenges by participant %v: %w", key, err)
	}

	var challenges []ChallengeEntity
	for _, row := range rows {
		challenge, err := mapChallengeFromRow(row)
		if err != nil {
			return nil, err
		}
		challenges = append(challenges, challenge)
	}

	slog.InfoContext(ctx, "got challenges by participant", "challengeKey", key, "since", since, "challenges", challenges)
	return challenges, nil
}

type DeleteResult struct {
	ChallengerID int64
	ChallengeeID int64
	Mode         GameMode
	FirstColor   Color
}

func DeleteChallenge(ctx context.Context, query *db.Queries, key ChallengeKey) (DeleteResult, error) {
	var dr DeleteResult
	row, err := query.DeleteChallenge(ctx, db.DeleteChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	if errors.Is(err, sql.ErrNoRows) {
		return dr, ErrChallengeNotFound
	}
	if err != nil {
		return dr, fmt.Errorf("delete challenge %d: %w", key, err)
	}

	startColor := Colors.Parse(string(row.StartColor))
	if startColor == nil {
		return dr, MakeColorError(string(row.Mode))
	}
	mode := Modes.Parse(string(row.Mode))
	if mode == nil {
		return dr, MakeGameModeError(string(row.Mode))
	}

	dr = DeleteResult{
		ChallengerID: row.ChallengerID,
		ChallengeeID: row.ChallengeeID,
		Mode:         *mode,
		FirstColor:   *startColor,
	}
	slog.InfoContext(ctx, "deleted challenge", "challengeKey", key, "dr", dr, "err", err)
	return dr, err
}

func DeleteExpiredChallenges(ctx context.Context, query *db.Queries, userID int64, threshold time.Duration) error {
	return DeleteExpiredChallengesOn(ctx, query, userID, time.Now().Add(-threshold))
}

func DeleteExpiredChallengesOn(ctx context.Context, query *db.Queries, userID int64, t time.Time) error {
	err := query.DeleteExpiredChallenges(ctx, db.DeleteExpiredChallengesParams{
		UserID: userID,
		Before: pgtype.Timestamptz{Valid: true, Time: t},
	})
	logutil.DynLog(ctx, "deleted expired challenges", err, "userID", userID, "expireTime", t)
	return nil
}

package svc

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/net/context"
	"hexchess-svc/db"
	"log/slog"
	"time"
)

type ChallengeEntity struct {
	ChallengerId      int64
	ChallengerName    string
	ChallengerCountry string
	ChallengerElo     float64
	ChallengeeId      int64
	ChallengeeName    string
	ChallengeeCountry string
	ChallengeeElo     float64
	TimeControl       string
	StartColor        string // from challenger's perspective
	MadeOn            time.Time
}

var (
	ErrDuplicateChallenge  = errors.New("duplicate challenge")
	ErrSelfChallenge       = errors.New("cannot challenge yourself")
	ErrParticipantConflict = errors.New("user already in a challenge")
)

const ExpireChallengeThreshold = time.Hour * 24 * 7

type ChallengeInst struct {
	ChallengerID int64     `json:"challengerId"`
	ChallengeeID int64     `json:"challengeeId"`
	TimeControl  string    `json:"timeControl"`
	StartColor   string    `json:"startColor"`
	MadeOn       time.Time `json:"madeOn"`
}

func mapChallengeFromRow(row db.SelectChallengesByParticipantRow) ChallengeEntity {
	return ChallengeEntity{
		ChallengerId:      row.ChallengerID,
		ChallengerName:    row.ChallengerName,
		ChallengerCountry: row.ChallengerCountry.String,
		ChallengerElo:     row.ChallengerElo,
		ChallengeeId:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry.String,
		ChallengeeElo:     row.ChallengeeElo,
		TimeControl:       row.TimeControl,
		StartColor:        row.StartColor,
		MadeOn:            row.MadeOn.Time,
	}
}

func InsertChallenge(ctx context.Context, q *db.Queries, inst ChallengeInst) error {
	_, err := InsertChallengeRet(ctx, q, inst)
	return err
}

func InsertChallengeRet(ctx context.Context, q *db.Queries, inst ChallengeInst) (ChallengeEntity, error) {
	trace := ctx.Value(TraceKey)

	if inst.MadeOn.IsZero() {
		inst.MadeOn = time.Now()
	}

	row, err := q.InsertChallenge(ctx, db.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		TimeControl:  inst.TimeControl,
		StartColor:   inst.StartColor,
		MadeOn:       pgtype.Timestamp{Valid: true, Time: inst.MadeOn},
	})

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			slog.Info("challenge already exists", "challenge", inst, "err", pgErr, "trace", trace)
			return ChallengeEntity{}, ErrDuplicateChallenge
		case "23503", "23506":
			slog.Info("violating key constraint when creating challenge", "challenge", inst, "err", pgErr, "trace", trace)
			return ChallengeEntity{}, ErrParticipantConflict
		}
	}

	challenge := mapChallengeFromRow(db.SelectChallengesByParticipantRow(row))

	dynLog("created a new challenge", err, "challenge", inst, "challenge", challenge, "trace", trace)
	return challenge, err
}

func GetChallengesByParticipant(ctx context.Context, q *db.Queries, challengerID *int64, challengeeID *int64, threshold time.Duration) ([]ChallengeEntity, error) {
	trace := ctx.Value(TraceKey)

	var pgChallengerID pgtype.Int8
	if challengerID != nil {
		pgChallengerID.Valid = true
		pgChallengerID.Int64 = *challengerID
	}
	var pgChallengeeID pgtype.Int8
	if challengeeID != nil {
		pgChallengeeID.Valid = true
		pgChallengeeID.Int64 = *challengeeID
	}

	t := time.Now().Add(-threshold)
	rows, err := q.SelectChallengesByParticipant(ctx, db.SelectChallengesByParticipantParams{
		ChallengerID: pgChallengerID,
		ChallengeeID: pgChallengeeID,
		Since:        pgtype.Timestamp{Valid: true, Time: t},
	})
	if err != nil {
		slog.Error("failed to get challenges by participant", "challengerID", challengerID, "challengeeID", challengeeID, "err", err, "trace", trace)
		return nil, fmt.Errorf("failed to get challenges by participant: %w", err)
	}

	var challenges []ChallengeEntity
	for _, row := range rows {
		challenges = append(challenges, mapChallengeFromRow(row))
	}

	return challenges, nil
}

type DeleteResult struct {
	ChallengerID int64
	ChallengeeID int64
	TimeControl  string
	StartColor   string
}

func DeleteChallenge(ctx context.Context, q *db.Queries, challengeeID int64, challengerID int64) (DeleteResult, error) {
	trace := ctx.Value(TraceKey)

	row, err := q.DeleteChallenge(ctx, db.DeleteChallengeParams{ChallengerID: challengeeID, ChallengeeID: challengerID})
	if err != nil {
		slog.Error("failed to create challenge", "err", err, "trace", trace)
		return DeleteResult{}, err
	}

	slog.Info("created a new challenge", "challengee", challengeeID, "challengerID", challengerID, "row", row, "err", err, "trace", trace)
	return DeleteResult(row), err
}

func DeleteExpiredChallenges(ctx context.Context, q *db.Queries, userID int64, threshold time.Duration) error {
	t := time.Now().Add(-threshold)
	err := q.DeleteExpiredChallenges(ctx, db.DeleteExpiredChallengesParams{
		UserID:     userID,
		ExpireTime: pgtype.Timestamp{Valid: true, Time: t},
	})
	dynLog("deleted expired challenges", err, "userID", userID, "expireTime", t, "trace", ctx.Value(TraceKey))
	return nil
}

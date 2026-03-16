package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/pkg/logutil"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
	Mode              string    `json:"mode"`
	StartColor        string    `json:"startColor"` // from challenger's perspective
	MadeOn            time.Time `json:"madeOn"`
	ExpiresOn         time.Time `json:"expiresOn"`
}

var (
	ErrDuplicateChallenge  = errors.New("duplicate challenge")
	ErrSelfChallenge       = errors.New("cannot challenge yourself")
	ErrParticipantConflict = errors.New("one or more participants are invalid")
	ErrChallengeNotFound   = errors.New("challenge not found")
)

const ExpireChallengeMaxAge = time.Hour * 24 * 7

type ChallengeInst struct {
	ChallengerID int64     `json:"challengerId"`
	ChallengeeID int64     `json:"challengeeId"`
	Mode         GameMode  `json:"mode"`
	StartColor   Color     `json:"startColor"`
	MadeOn       time.Time `json:"madeOn"`
}

func mapChallengeFromRow(row sqlc.SelectChallengesByParticipantRow) ChallengeEntity {
	return ChallengeEntity{
		ChallengerID:      row.ChallengerID,
		ChallengerName:    row.ChallengerName,
		ChallengerCountry: row.ChallengerCountry,
		ChallengerElo:     defaultElo(row.ChallengerElo),
		ChallengeeID:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry,
		ChallengeeElo:     defaultElo(row.ChallengeeElo),
		Mode:              string(row.Mode),
		StartColor:        string(row.StartColor),
		MadeOn:            row.MadeOn.Time,
		ExpiresOn:         row.MadeOn.Time.Add(ExpireChallengeMaxAge),
	}
}

func (svc *Services) InsertChallenge(ctx context.Context, inst ChallengeInst) error {
	_, err := svc.InsertChallengeRet(ctx, inst)
	return err
}

func mapChallengeInsertErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case db.ErrPgUniqueViolation:
		return ErrDuplicateChallenge
	case db.ErrPgForeignKeyViolation, db.ErrPgCheckViolation:
		return ErrParticipantConflict
	default:
		return err
	}
}

func (svc *Services) InsertChallengeRet(ctx context.Context, inst ChallengeInst) (challenge ChallengeEntity, err error) {
	if inst.ChallengerID == inst.ChallengeeID {
		return challenge, ErrSelfChallenge
	}
	if inst.MadeOn.IsZero() {
		inst.MadeOn = time.Now()
	}

	row, dbErr := svc.Queries.InsertChallenge(ctx, sqlc.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		Mode:         sqlc.ModeEnum(inst.Mode.String()),
		StartColor:   sqlc.ColorEnum(inst.StartColor.String()),
		MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
	})
	if dbErr != nil {
		svcErr := mapChallengeInsertErr(dbErr)
		slog.WarnContext(ctx, "failed to insert challenge with db error", "dbErr", dbErr, "svcErr", svcErr)
		return challenge, svcErr
	}
	challenge = mapChallengeFromRow(sqlc.SelectChallengesByParticipantRow(row))

	slog.InfoContext(ctx, "created a new challenge", "challenge", inst, "challenge", challenge)
	return challenge, nil
}

type ChallengeKey struct {
	ChallengerID int64
	ChallengeeID int64
}

// GetChallengesByParticipant will select challenges by the participant after the 'since' time
func (svc *Services) GetChallengesByParticipant(ctx context.Context, key ChallengeKey) ([]ChallengeEntity, error) {
	since := svc.EntropySource.GetNow().Add(-ExpireChallengeMaxAge)

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

	rows, err := svc.Queries.SelectChallengesByParticipant(ctx, sqlc.SelectChallengesByParticipantParams{
		ChallengerID: pgChallengerID,
		ChallengeeID: pgChallengeeID,
		Since:        pgtype.Timestamptz{Valid: true, Time: since},
	})
	if err != nil {
		return nil, fmt.Errorf("get challenges by participant %v: %w", key, err)
	}

	var challenges []ChallengeEntity
	for _, row := range rows {
		challenges = append(challenges, mapChallengeFromRow(row))
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

func (svc *Services) DeleteChallenge(ctx context.Context, key ChallengeKey) (delResult DeleteResult, err error) {
	row, err := svc.Queries.DeleteChallenge(ctx, sqlc.DeleteChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return delResult, ErrChallengeNotFound
	}
	if err != nil {
		return delResult, fmt.Errorf("delete challenge %d: %w", key, err)
	}

	p := EnumParser{}
	startColor := p.Color(string(row.StartColor))
	mode := p.GameMode(string(row.Mode))

	if err := p.Err(); err != nil {
		return delResult, err
	}

	delResult = DeleteResult{
		ChallengerID: row.ChallengerID,
		ChallengeeID: row.ChallengeeID,
		Mode:         mode,
		FirstColor:   startColor,
	}
	slog.InfoContext(ctx, "deleted challenge", "challengeKey", key, "dr", delResult, "err", err)
	return delResult, err
}

func (svc *Services) DeleteExpiredChallenges(ctx context.Context, userID int64) error {
	t := svc.EntropySource.GetNow().Add(-ExpireChallengeMaxAge)
	err := svc.Queries.DeleteExpiredChallenges(ctx, sqlc.DeleteExpiredChallengesParams{
		UserID: userID,
		Before: pgtype.Timestamptz{Valid: true, Time: t},
	})
	logutil.DynLog(ctx, "deleted expired challenges", err, "userID", userID, "expireTime", t)
	return nil
}

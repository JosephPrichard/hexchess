package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/domain"
	"hexchess-svc/util/enum"
	"log/slog"
	"time"

	"hexchess-svc/db/sqlc"
	"hexchess-svc/util/logutil"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrDuplicateChallenge     = errors.New("duplicate challenge")
	ErrSelfChallenge          = errors.New("cannot challenge yourself")
	ErrInvalidChallengeMember = errors.New("one or more challenge participants are invalid")
	ErrChallengeNotFound      = errors.New("challenge not found")
)

const ExpireChallengeMaxAge = time.Hour * 24 * 7

type ChallengeInst struct {
	ChallengerID int64            `json:"challengerId"`
	ChallengeeID int64            `json:"challengeeId"`
	Mode         domain.GameMode  `json:"mode"`
	StartColor   domain.GameColor `json:"startColor"`
	MadeOn       time.Time        `json:"madeOn"`
}

func (svc *HexchessServices) InsertChallenge(ctx context.Context, inst ChallengeInst) (domain.Challenge, error) {
	if inst.ChallengerID == inst.ChallengeeID {
		return domain.Challenge{}, ErrSelfChallenge
	}
	if inst.MadeOn.IsZero() {
		inst.MadeOn = time.Now()
	}

	row, dbErr := svc.querier.InsertChallenge(ctx, sqlc.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		Mode:         sqlc.ModeEnum(inst.Mode.String()),
		StartColor:   sqlc.ColorEnum(inst.StartColor.String()),
		MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
	})
	if dbErr != nil {
		if svcErr := mapChallengeInsertErr(dbErr); svcErr != nil {
			return domain.Challenge{}, svcErr
		}
		return domain.Challenge{}, fmt.Errorf("insert challenge %+v: %w", inst, dbErr)
	}

	challenge := mapChallengeRow(sqlc.SelectChallengesByParticipantRow(row))

	slog.InfoContext(ctx, "created a new challenge", "challenge", inst, "challenge", challenge)
	return challenge, nil
}

func mapChallengeInsertErr(err error) error {
	return mapInsertErr(err, ErrDuplicateChallenge, ErrInvalidChallengeMember)
}

type ChallengeKey struct {
	ChallengerID int64
	ChallengeeID int64
}

// GetChallengesByParticipant will select challenges by the participant after the 'since' time
func (svc *HexchessServices) GetChallengesByParticipant(ctx context.Context, key ChallengeKey) ([]domain.Challenge, error) {
	since := svc.entropy.GetTime().Add(-ExpireChallengeMaxAge)

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

	rows, err := svc.querier.SelectChallengesByParticipant(ctx, sqlc.SelectChallengesByParticipantParams{
		ChallengerID: pgChallengerID,
		ChallengeeID: pgChallengeeID,
		Since:        pgtype.Timestamptz{Valid: true, Time: since},
	})
	if err != nil {
		return nil, fmt.Errorf("get challenges by participant %v: %w", key, err)
	}

	challenges := make([]domain.Challenge, 0, len(rows))
	for _, row := range rows {
		challenges = append(challenges, mapChallengeRow(row))
	}

	slog.InfoContext(ctx, "got challenges by participant", "challengeKey", key, "since", since, "challenges", challenges)
	return challenges, nil
}

type DeleteResult struct {
	ChallengerID int64
	ChallengeeID int64
	Mode         domain.GameMode
	FirstColor   domain.GameColor
}

func (svc *HexchessServices) DeleteChallenge(ctx context.Context, key ChallengeKey) (DeleteResult, error) {
	challengeRow, err := svc.querier.DeleteChallenge(ctx, sqlc.DeleteChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	if IsErrNoRows(err) {
		return DeleteResult{}, ErrChallengeNotFound
	} else if err != nil {
		return DeleteResult{}, fmt.Errorf("delete challenge %d: %w", key, err)
	}

	gameColor := enum.Expect(challengeRow.StartColor, domain.GameColorEnums)
	gameMode := enum.Expect(challengeRow.Mode, domain.GameModeEnums)

	delResult := DeleteResult{
		ChallengerID: challengeRow.ChallengerID,
		ChallengeeID: challengeRow.ChallengeeID,
		Mode:         gameMode,
		FirstColor:   gameColor,
	}
	slog.InfoContext(ctx, "deleted challenge", "challengeKey", key, "dr", delResult, "err", err)
	return delResult, err
}

func (svc *HexchessServices) DeleteExpiredChallenges(ctx context.Context, userID int64) error {
	beforeTime := svc.entropy.GetTime().Add(-ExpireChallengeMaxAge)

	err := svc.querier.DeleteExpiredChallenges(ctx, sqlc.DeleteExpiredChallengesParams{
		UserID: userID,
		Before: pgtype.Timestamptz{Valid: true, Time: beforeTime},
	})
	logutil.DynLog(ctx, "deleted expired challenges", err, "userID", userID, "expireTime", beforeTime)
	return nil
}

func mapChallengeRow(row sqlc.SelectChallengesByParticipantRow) domain.Challenge {
	gameColor := enum.Expect(row.StartColor, domain.GameColorEnums)
	gameMode := enum.Expect(row.Mode, domain.GameModeEnums)

	return domain.Challenge{
		ChallengerID:      row.ChallengerID,
		ChallengerName:    row.ChallengerName,
		ChallengerCountry: row.ChallengerCountry,
		ChallengerElo:     domain.DefaultUserElo(row.ChallengerElo),
		ChallengeeID:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry,
		ChallengeeElo:     domain.DefaultUserElo(row.ChallengeeElo),
		Mode:              gameMode,
		StartColor:        gameColor,
		MadeOn:            row.MadeOn.Time,
		ExpiresOn:         row.MadeOn.Time.Add(ExpireChallengeMaxAge),
	}
}

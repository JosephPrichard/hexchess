package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/util/enum"
	"log/slog"
	"time"

	"hexchess-svc/db/sqlc"
	"hexchess-svc/util/logutil"

	"github.com/jackc/pgx/v5/pgtype"
)

type ChallengeDTO struct {
	ChallengerID      int64     `json:"challengerId"`
	ChallengerName    string    `json:"challengerName"`
	ChallengerCountry string    `json:"challengerCountry"`
	ChallengerElo     float64   `json:"challengerElo"`
	ChallengeeID      int64     `json:"challengeeId"`
	ChallengeeName    string    `json:"challengeeName"`
	ChallengeeCountry string    `json:"challengeeCountry"`
	ChallengeeElo     float64   `json:"challengeeElo"`
	Mode              GameMode  `json:"mode"`
	StartColor        GameColor `json:"startColor"` // from challenger's perspective
	MadeOn            time.Time `json:"madeOn"`
	ExpiresOn         time.Time `json:"expiresOn"`
}

var (
	ErrDuplicateChallenge     = errors.New("duplicate challenge")
	ErrSelfChallenge          = errors.New("cannot challenge yourself")
	ErrInvalidChallengeMember = errors.New("one or more challenge participants are invalid")
	ErrChallengeNotFound      = errors.New("challenge not found")
)

const ExpireChallengeMaxAge = time.Hour * 24 * 7

type ChallengeInst struct {
	ChallengerID int64     `json:"challengerId"`
	ChallengeeID int64     `json:"challengeeId"`
	Mode         GameMode  `json:"mode"`
	StartColor   GameColor `json:"startColor"`
	MadeOn       time.Time `json:"madeOn"`
}

func (svc *HexchessServices) InsertChallenge(ctx context.Context, inst ChallengeInst) error {
	_, err := svc.InsertChallengeRet(ctx, inst)
	return err
}

func (svc *HexchessServices) InsertChallengeRet(ctx context.Context, inst ChallengeInst) (ChallengeDTO, error) {
	if inst.ChallengerID == inst.ChallengeeID {
		return ChallengeDTO{}, ErrSelfChallenge
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
			return ChallengeDTO{}, svcErr
		}
		return ChallengeDTO{}, fmt.Errorf("insert challenge %+v: %w", inst, dbErr)
	}

	challenge, err := mapChallengeRow(sqlc.SelectChallengesByParticipantRow(row))
	if err != nil {
		return ChallengeDTO{}, fmt.Errorf("map challenge from row: %w", err)
	}
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
func (svc *HexchessServices) GetChallengesByParticipant(ctx context.Context, key ChallengeKey) ([]ChallengeDTO, error) {
	since := svc.entropy.GetNow().Add(-ExpireChallengeMaxAge)

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

	challenges := make([]ChallengeDTO, 0, len(rows))
	for _, row := range rows {
		challenge, err := mapChallengeRow(row)
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
	FirstColor   GameColor
}

func (svc *HexchessServices) DeleteChallenge(ctx context.Context, key ChallengeKey) (DeleteResult, error) {
	challengeRow, err := svc.querier.DeleteChallenge(ctx, sqlc.DeleteChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	if IsErrNoRows(err) {
		return DeleteResult{}, ErrChallengeNotFound
	} else if err != nil {
		return DeleteResult{}, fmt.Errorf("delete challenge %d: %w", key, err)
	}

	gameColor, colorErr := enum.Parse(challengeRow.StartColor, GameColorEnums)
	gameMode, modeErr := enum.Parse(challengeRow.Mode, GameModeEnums)
	if err := errors.Join(colorErr, modeErr); err != nil {
		return DeleteResult{}, err
	}

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
	t := svc.entropy.GetNow().Add(-ExpireChallengeMaxAge)
	err := svc.querier.DeleteExpiredChallenges(ctx, sqlc.DeleteExpiredChallengesParams{
		UserID: userID,
		Before: pgtype.Timestamptz{Valid: true, Time: t},
	})
	logutil.DynLog(ctx, "deleted expired challenges", err, "userID", userID, "expireTime", t)
	return nil
}

func mapChallengeRow(row sqlc.SelectChallengesByParticipantRow) (ChallengeDTO, error) {
	gameColor, colorErr := enum.Parse(row.StartColor, GameColorEnums)
	gameMode, modeErr := enum.Parse(row.Mode, GameModeEnums)
	if err := errors.Join(colorErr, modeErr); err != nil {
		return ChallengeDTO{}, err
	}

	return ChallengeDTO{
		ChallengerID:      row.ChallengerID,
		ChallengerName:    row.ChallengerName,
		ChallengerCountry: row.ChallengerCountry,
		ChallengerElo:     defaultElo(row.ChallengerElo),
		ChallengeeID:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry,
		ChallengeeElo:     defaultElo(row.ChallengeeElo),
		Mode:              gameMode,
		StartColor:        gameColor,
		MadeOn:            row.MadeOn.Time,
		ExpiresOn:         row.MadeOn.Time.Add(ExpireChallengeMaxAge),
	}, nil
}

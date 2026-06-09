package svc

import (
	"context"
	"errors"
	"hexchess-svc/db"
	"hexchess-svc/lib/enum"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"log/slog"
	"time"

	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/logutil"

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
	ChallengerID int64           `json:"challengerId"`
	ChallengeeID int64           `json:"challengeeId"`
	Mode         model.GameMode  `json:"mode"`
	StartColor   model.GameColor `json:"startColor"`
	MadeOn       time.Time       `json:"madeOn"`
}

func (services *HexchessServices) InsertChallenge(ctx context.Context, inst ChallengeInst) (model.Challenge, error) {
	if inst.ChallengerID == inst.ChallengeeID {
		return model.Challenge{}, ErrSelfChallenge
	}
	if inst.MadeOn.IsZero() {
		inst.MadeOn = time.Now()
	}

	row, dbErr := services.querier.InsertChallenge(ctx, sqlc.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		Mode:         sqlc.ModeEnum(inst.Mode.String()),
		StartColor:   sqlc.ColorEnum(inst.StartColor.String()),
		MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
	})
	if dbErr != nil {
		if svcErr := mapChallengeInsertErr(dbErr); svcErr != nil {
			return model.Challenge{}, svcErr
		}
		return model.Challenge{}, serrors.New("insert challenge", dbErr, "inst", inst)
	}

	challenge := mapChallengeRow(sqlc.SelectChallengesByParticipantRow(row))

	slog.InfoContext(ctx, "created a new challenge", "challenge", inst, "challenge", challenge)
	return challenge, nil
}

func (services *HexchessServices) BatchInsertChallenges(ctx context.Context, insts []ChallengeInst) error {
	batches := make([]sqlc.BatchInsertChallengeParams, 0, len(insts))

	for _, inst := range insts {
		if inst.MadeOn.IsZero() {
			inst.MadeOn = time.Now()
		}
		batches = append(batches, sqlc.BatchInsertChallengeParams{
			ChallengerID: inst.ChallengerID,
			ChallengeeID: inst.ChallengeeID,
			Mode:         sqlc.ModeEnum(inst.Mode.String()),
			StartColor:   sqlc.ColorEnum(inst.StartColor.String()),
			MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
		})
	}
	var insertErrs []error

	services.querier.BatchInsertChallenge(ctx, batches).Exec(func(i int, err error) {
		if err != nil {
			insertErrs = append(insertErrs, serrors.New("batch insert challenge", err, "index", i))
		}
	})
	err := errors.Join(insertErrs...)

	logutil.Log(ctx, "batch inserted challenges", err, "insts", insts)
	return err
}

func mapChallengeInsertErr(err error) error {
	return db.MapInsertErr(err, ErrDuplicateChallenge, ErrInvalidChallengeMember)
}

type ChallengeKey struct {
	ChallengerID int64
	ChallengeeID int64
}

// GetChallengesByParticipant will select challenges by the participant after the 'since' time
func (services *HexchessServices) GetChallengesByParticipant(ctx context.Context, key ChallengeKey) ([]model.Challenge, error) {
	since := services.entropy.GetTime().Add(-ExpireChallengeMaxAge)

	rows, err := services.querier.SelectChallengesByParticipant(ctx, sqlc.SelectChallengesByParticipantParams{
		ChallengerID: db.OptInt8(key.ChallengerID),
		ChallengeeID: db.OptInt8(key.ChallengeeID),
		Since:        pgtype.Timestamptz{Valid: true, Time: since},
	})
	if err != nil {
		return nil, serrors.New("get challenges by participant", err, "key", key)
	}

	challenges := make([]model.Challenge, 0, len(rows))
	for _, row := range rows {
		challenges = append(challenges, mapChallengeRow(row))
	}

	slog.InfoContext(ctx, "got challenges by participant", "challengeKey", key, "since", since, "challenges", challenges)
	return challenges, nil
}

type DeleteResult struct {
	ChallengerID int64
	ChallengeeID int64
	Mode         model.GameMode
	FirstColor   model.GameColor
}

func (services *HexchessServices) DeleteChallenge(ctx context.Context, key ChallengeKey) (DeleteResult, error) {
	challengeRow, err := services.querier.DeleteChallenge(ctx, sqlc.DeleteChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	if db.IsErrNoRows(err) {
		return DeleteResult{}, ErrChallengeNotFound
	} else if err != nil {
		return DeleteResult{}, serrors.New("delete challenge", err, "key", key)
	}

	gameColor := enum.Expect(challengeRow.StartColor, model.GameColorEnums)
	gameMode := enum.Expect(challengeRow.Mode, model.GameModeEnums)

	delResult := DeleteResult{
		ChallengerID: challengeRow.ChallengerID,
		ChallengeeID: challengeRow.ChallengeeID,
		Mode:         gameMode,
		FirstColor:   gameColor,
	}
	slog.InfoContext(ctx, "deleted challenge", "challengeKey", key, "dr", delResult, "error", err)
	return delResult, err
}

func (services *HexchessServices) DeleteExpiredChallenges(ctx context.Context, userID int64) error {
	// TODO: call this from a cronjob to clear out expired challenges every couple days
	beforeTime := services.entropy.GetTime().Add(-ExpireChallengeMaxAge)

	err := services.querier.DeleteExpiredChallenges(ctx, sqlc.DeleteExpiredChallengesParams{
		UserID: userID,
		Before: pgtype.Timestamptz{Valid: true, Time: beforeTime},
	})
	logutil.Log(ctx, "deleted expired challenges", err, "userID", userID, "expireTime", beforeTime)
	return nil
}

func (services *HexchessServices) CountUserChallenges(ctx context.Context, userID int64) (int64, error) {
	return services.querier.CountReceivedChallenges(ctx, userID)
}

func mapChallengeRow(row sqlc.SelectChallengesByParticipantRow) model.Challenge {
	gameColor := enum.Expect(row.StartColor, model.GameColorEnums)
	gameMode := enum.Expect(row.Mode, model.GameModeEnums)

	return model.Challenge{
		ChallengerID:      row.ChallengerID,
		ChallengerName:    row.ChallengerName,
		ChallengerCountry: row.ChallengerCountry,
		ChallengerElo:     model.DefaultUserElo(row.ChallengerElo),
		ChallengeeID:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry,
		ChallengeeElo:     model.DefaultUserElo(row.ChallengeeElo),
		Mode:              gameMode,
		StartColor:        gameColor,
		MadeOn:            row.MadeOn.Time,
		ExpiresOn:         row.MadeOn.Time.Add(ExpireChallengeMaxAge),
	}
}

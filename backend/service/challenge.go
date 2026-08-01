package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/db/mutator"
	"hexchess-svc/db/query"
	"hexchess-svc/model"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"hexchess-svc/utils/logutil"

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
	defer perf.WithContext(ctx).Log()

	if inst.ChallengerID == inst.ChallengeeID {
		return model.Challenge{}, ErrSelfChallenge
	}
	if inst.MadeOn.IsZero() {
		inst.MadeOn = time.Now()
	}

	row, dbErr := services.querier.InsertChallenge(ctx, mutator.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		Mode:         mutator.ModeEnum(inst.Mode.String()),
		StartColor:   mutator.ColorEnum(inst.StartColor.String()),
		MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
	})
	if dbErr != nil {
		if svcErr := mapChallengeInsertErr(dbErr); svcErr != nil {
			return model.Challenge{}, svcErr
		}
		return model.Challenge{}, serrors.New("insert challenge", dbErr, "inst", inst)
	}

	challenge := model.Challenge{
		ChallengerID:      row.ChallengerID,
		ChallengerName:    row.ChallengerName,
		ChallengerCountry: row.ChallengerCountry,
		ChallengerElo:     model.DefaultUserElo(row.ChallengerElo),
		ChallengeeID:      row.ChallengeeID,
		ChallengeeName:    row.ChallengeeName,
		ChallengeeCountry: row.ChallengeeCountry,
		ChallengeeElo:     model.DefaultUserElo(row.ChallengeeElo),
		Mode:              enum.Expect(row.Mode, model.GameModeEnums),
		StartColor:        enum.Expect(row.StartColor, model.GameColorEnums),
		MadeOn:            row.MadeOn.Time,
		ExpiresOn:         row.MadeOn.Time.Add(ExpireChallengeMaxAge),
	}

	slog.InfoContext(ctx, "created a new challenge", "challenge", inst, "challenge", challenge)
	return challenge, nil
}

func (services *HexchessServices) BatchInsertChallenges(ctx context.Context, insts []ChallengeInst) error {
	batches := make([]mutator.BatchInsertChallengeParams, 0, len(insts))

	for _, inst := range insts {
		if inst.MadeOn.IsZero() {
			inst.MadeOn = time.Now()
		}
		batches = append(batches, mutator.BatchInsertChallengeParams{
			ChallengerID: inst.ChallengerID,
			ChallengeeID: inst.ChallengeeID,
			Mode:         mutator.ModeEnum(inst.Mode.String()),
			StartColor:   mutator.ColorEnum(inst.StartColor.String()),
			MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
		})
	}

	slog.InfoContext(ctx, "batch inserting challenges", "insts", insts)

	var insertErrs []error

	services.querier.BatchInsertChallenge(ctx, batches).Exec(func(i int, err error) {
		if err != nil {
			insertErrs = append(insertErrs, fmt.Errorf("batch insert challenge with inst %+v: %w", insts[i], err))
		}
	})
	err := errors.Join(insertErrs...)

	logutil.Log(ctx, "batch inserted challenges", err)
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
	defer perf.WithContext(ctx).Log()

	since := services.entropy.GetTime().Add(-ExpireChallengeMaxAge)

	rows, err := services.readQuerier.SelectChallengesByParticipant(ctx, query.SelectChallengesByParticipantParams{
		ChallengerID: db.OptInt8(key.ChallengerID),
		ChallengeeID: db.OptInt8(key.ChallengeeID),
		Since:        pgtype.Timestamptz{Valid: true, Time: since},
	})
	if err != nil {
		return nil, serrors.New("get challenges by participant", err, "key", key)
	}

	challenges := make([]model.Challenge, 0, len(rows))
	for _, row := range rows {
		challenges = append(challenges, model.Challenge{
			ChallengerID:      row.ChallengerID,
			ChallengerName:    row.ChallengerName,
			ChallengerCountry: row.ChallengerCountry,
			ChallengerElo:     model.DefaultUserElo(row.ChallengerElo),
			ChallengeeID:      row.ChallengeeID,
			ChallengeeName:    row.ChallengeeName,
			ChallengeeCountry: row.ChallengeeCountry,
			ChallengeeElo:     model.DefaultUserElo(row.ChallengeeElo),
			Mode:              enum.Expect(row.Mode, model.GameModeEnums),
			StartColor:        enum.Expect(row.StartColor, model.GameColorEnums),
			MadeOn:            row.MadeOn.Time,
			ExpiresOn:         row.MadeOn.Time.Add(ExpireChallengeMaxAge),
		})
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

func (services *HexchessServices) DeleteChallenge(ctx context.Context, challengerID int64, challengeeID int64) (DeleteResult, error) {
	defer perf.WithContext(ctx).Log()

	params := mutator.DeleteChallengeParams{ChallengerID: challengerID, ChallengeeID: challengeeID}

	challengeRow, err := services.querier.DeleteChallenge(ctx, params)
	if db.IsErrNoRows(err) {
		return DeleteResult{}, ErrChallengeNotFound
	} else if err != nil {
		return DeleteResult{}, serrors.New("delete challenge", err, "params", params)
	}

	gameColor := enum.Expect(challengeRow.StartColor, model.GameColorEnums)
	gameMode := enum.Expect(challengeRow.Mode, model.GameModeEnums)

	delResult := DeleteResult{
		ChallengerID: challengeRow.ChallengerID,
		ChallengeeID: challengeRow.ChallengeeID,
		Mode:         gameMode,
		FirstColor:   gameColor,
	}
	slog.InfoContext(ctx, "deleted challenge", "params", params, "deleteResult", delResult, "error", err)
	return delResult, err
}

func (services *HexchessServices) DeleteExpiredChallenges(ctx context.Context, userID int64) error {
	defer perf.WithContext(ctx).Log()

	// TODO: call this from a cronjob to clear out expired challenges every couple days
	beforeTime := services.entropy.GetTime().Add(-ExpireChallengeMaxAge)

	err := services.querier.DeleteExpiredChallenges(ctx, mutator.DeleteExpiredChallengesParams{
		UserID: userID,
		Before: pgtype.Timestamptz{Valid: true, Time: beforeTime},
	})
	logutil.Log(ctx, "deleted expired challenges", err, "userID", userID, "expireTime", beforeTime)
	return nil
}

func (services *HexchessServices) CountUserChallenges(ctx context.Context, userID int64) (int64, error) {
	return services.querier.CountReceivedChallenges(ctx, userID)
}

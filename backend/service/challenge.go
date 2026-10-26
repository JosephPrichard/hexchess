package service

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/database"
	"hexchess-svc/database/mutate"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"hexchess-svc/utils/slogutil"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrDuplicateChallenge     = errors.New("duplicate challenge")
	ErrSelfChallenge          = errors.New("cannot challenge yourself")
	ErrInvalidChallengeMember = errors.New("one or more challenge participants are invalid")
	ErrChallengeNotFound      = errors.New("challenge not found")
)

const ExpireChallengeMaxAge = time.Hour * 24 * 7

type ChallengeService struct {
	database.Database
	entropy entropy.Generator
}

func NewChallengeService(database database.Database, entropy entropy.Generator) *ChallengeService {
	return &ChallengeService{Database: database, entropy: entropy}
}

type ChallengeInst struct {
	ChallengerID int64           `json:"challengerId"`
	ChallengeeID int64           `json:"challengeeId"`
	Mode         model.GameMode  `json:"mode"`
	StartColor   model.GameColor `json:"startColor"`
	MadeOn       time.Time       `json:"madeOn"`
}

func (service *ChallengeService) InsertChallenge(ctx context.Context, inst ChallengeInst) (_ model.Challenge, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	if inst.ChallengerID == inst.ChallengeeID {
		return model.Challenge{}, ErrSelfChallenge
	}
	if inst.MadeOn.IsZero() {
		inst.MadeOn = time.Now()
	}

	row, err := service.QuerierMutator.InsertChallenge(ctx, mutate.InsertChallengeParams{
		ChallengerID: inst.ChallengerID,
		ChallengeeID: inst.ChallengeeID,
		Mode:         mutate.ModeEnum(inst.Mode.String()),
		StartColor:   mutate.ColorEnum(inst.StartColor.String()),
		MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
	})
	if err != nil {
		if svcErr := mapChallengeInsertErr(err); svcErr != nil {
			return model.Challenge{}, svcErr
		}
		return model.Challenge{}, serrors.New("insert challenge", err, "inst", inst)
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

func (service *ChallengeService) BatchInsertChallenges(ctx context.Context, insts []ChallengeInst) error {
	batches := make([]mutate.BatchInsertChallengeParams, 0, len(insts))

	for _, inst := range insts {
		if inst.MadeOn.IsZero() {
			inst.MadeOn = time.Now()
		}
		batches = append(batches, mutate.BatchInsertChallengeParams{
			ChallengerID: inst.ChallengerID,
			ChallengeeID: inst.ChallengeeID,
			Mode:         mutate.ModeEnum(inst.Mode.String()),
			StartColor:   mutate.ColorEnum(inst.StartColor.String()),
			MadeOn:       pgtype.Timestamptz{Valid: true, Time: inst.MadeOn},
		})
	}

	slog.InfoContext(ctx, "batch inserting challenges", "insts", insts)

	var insertErrs []error

	service.QuerierMutator.BatchInsertChallenge(ctx, batches).Exec(func(i int, err error) {
		if err != nil {
			insertErrs = append(insertErrs, fmt.Errorf("batch insert challenge with inst %+v: %w", insts[i], err))
		}
	})
	err := errors.Join(insertErrs...)

	if err == nil {
		slog.InfoContext(ctx, "batch inserted challenges")
	} else {
		slog.ErrorContext(ctx, "failed to batch inserted challenges", "error", err)
	}
	return err
}

func mapChallengeInsertErr(err error) error {
	return database.MapInsertErr(err, ErrDuplicateChallenge, ErrInvalidChallengeMember)
}

type Key struct {
	ChallengerID int64
	ChallengeeID int64
}

// GetChallengesByParticipant will select challenges by the participant after the 'since' time
func (service *ChallengeService) GetChallengesByParticipant(ctx context.Context, key Key) (_ []model.Challenge, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	since := service.entropy.GetTime().Add(-ExpireChallengeMaxAge)

	rows, err := service.Querier.SelectChallengesByParticipant(ctx, query.SelectChallengesByParticipantParams{
		ChallengerID: database.OptInt8(key.ChallengerID),
		ChallengeeID: database.OptInt8(key.ChallengeeID),
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

func (service *ChallengeService) DeleteChallenge(ctx context.Context, challengerID int64, challengeeID int64) (_ DeleteResult, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	params := mutate.DeleteChallengeParams{ChallengerID: challengerID, ChallengeeID: challengeeID}

	challengeRow, err := service.QuerierMutator.DeleteChallenge(ctx, params)
	if database.IsErrNoRows(err) {
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

func (service *ChallengeService) DeleteExpiredChallenges(ctx context.Context, userID int64) (err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	// TODO: call this from a cronjob to clear out expired challenges every couple days
	beforeTime := service.entropy.GetTime().Add(-ExpireChallengeMaxAge)

	err = service.QuerierMutator.DeleteExpiredChallenges(ctx, mutate.DeleteExpiredChallengesParams{
		UserID: userID,
		Before: pgtype.Timestamptz{Valid: true, Time: beforeTime},
	})
	slog.Log(ctx, slogutil.ErrorLevel(err), "deleted expired challenges", "error", err, "userID", userID, "expireTime", beforeTime)
	return nil
}

func (service *ChallengeService) CountUserChallenges(ctx context.Context, userID int64) (int64, error) {
	return service.Querier.CountReceivedChallenges(ctx, userID)
}

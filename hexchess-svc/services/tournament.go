package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/internal/enum"
	"log/slog"
	"math"
	"time"
)

type TournamentDTO struct {
	ID             int64            `json:"id"`
	TournamentKey  uuid.UUID        `json:"TournamentKey"`
	Name           string           `json:"name"`
	Depth          int32            `json:"depth"`
	MaxPlayerCount int32            `json:"maxPlayerCount"`
	ScheduledOn    time.Time        `json:"scheduledOn"`
	IsScheduled    bool             `json:"isScheduled"`
	CreatedOn      time.Time        `json:"createdOn"`
	CreatedBy      int64            `json:"createdBy"`
	Status         TournamentStatus `json:"status"`
	Mode           GameMode         `json:"mode"`
}

type ParticipantDTO struct {
	TournamentKey uuid.UUID `json:"TournamentKey"`
	JoinedOn      time.Time `json:"joinedOn"`
	LbdUserDTO              // fetches the leaderboard data for the mode the tournament is in.
}
type TournamentReplay struct {
	ReplayDTO
	ReplayViewDto
}

type MatchDTO struct {
	ID            int64             `json:"id"`
	GameID        string            `json:"gameId"`
	TournamentKey uuid.UUID         `json:"tournamentKey"`
	Depth         int32             `json:"depth"`
	CreatedOn     time.Time         `json:"createdOn"`
	Replay        *TournamentReplay `json:"replay"`
}

type FullTournamentDTO struct {
	Participants []ParticipantDTO `json:"participants"`
	Matches      []MatchDTO       `json:"matches"`
	TournamentDTO
}

const MaxTournamentDepth = 5 // equivalent to 32 players, 16 matches first round, 31 matches in total, 5 matches per player

func PlayersAtDepth(depth int32) int32 {
	return int32(math.Pow(2, float64(depth)))
}

func MatchesAtDepth(depth int32) int32 {
	return PlayersAtDepth(depth) / 2
}

type InvalidDepthError struct {
	actualDepth int32
}

func (e InvalidDepthError) Error() string {
	return fmt.Sprintf("invalid depth: %d, must be less than %d", e.actualDepth, MaxTournamentDepth)
}

type TournamentInst struct {
	Key         uuid.UUID     `json:"key"`
	Name        string        `json:"name"`
	Depth       int32         `json:"depth" json:"depth"`
	Mode        GameMode      `json:"mode" json:"mode"`
	ScheduledIn time.Duration `json:"scheduledIn"`
	CreatedOn   time.Time     `json:"createdOn"`
	CreatedBy   int64         `json:"createdBy"`
}

var InsertionStatus = TournamentLobby.String()

func (svc *Services) CreateTournament(ctx context.Context, inst TournamentInst) (int64, error) {
	if inst.Depth > MaxTournamentDepth {
		return 0, InvalidDepthError{actualDepth: inst.Depth}
	}

	insertionTime := svc.EntropySource.GetNow()

	isScheduledTournament := inst.ScheduledIn > 0
	scheduledOn := insertionTime.Add(inst.ScheduledIn)

	if inst.CreatedOn.IsZero() {
		inst.CreatedOn = insertionTime
	}

	tournamentID, err := svc.Querier.InsertTournament(ctx, sqlc.InsertTournamentParams{
		Tkey:        pgtype.UUID{Bytes: inst.Key, Valid: true},
		Name:        inst.Name,
		Depth:       inst.Depth,
		Status:      sqlc.TournamentStatusEnum(InsertionStatus),
		ScheduledOn: pgtype.Timestamptz{Time: scheduledOn, Valid: isScheduledTournament},
		CreatedOn:   pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		CreatedBy:   inst.CreatedBy,
		UpdatedOn:   pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		Mode:        sqlc.ModeEnum(inst.Mode.String()),
	})
	if err != nil {
		return 0, fmt.Errorf("insert tournament: %w", err)
	}

	slog.InfoContext(ctx, "created tournament", "TournamentKey", tournamentID)
	return tournamentID, nil
}

var ErrTournamentNotFound = fmt.Errorf("tournament does not exist")

func (svc *Services) getTournamentByID(ctx context.Context, tournamentKey uuid.UUID) (FullTournamentDTO, error) {
	pgKey := pgtype.UUID{Bytes: tournamentKey, Valid: true}

	var tournamentRow sqlc.SelectTournamentByIdRow
	var matchRows []sqlc.SelectMatchesByTournamentIdRow
	var participantRows []sqlc.SelectParticipantsByTournamentIdRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		tournamentRow, err = svc.Querier.SelectTournamentById(egCtx, pgKey)
		if IsErrNoRows(err) {
			return ErrTournamentNotFound
		} else if err != nil {
			return fmt.Errorf("select tournament by id %v: %w", tournamentKey, err)
		}
		return nil
	})
	eg.Go(func() (err error) {
		participantRows, err = svc.Querier.SelectParticipantsByTournamentId(egCtx, pgKey)
		if err != nil {
			return fmt.Errorf("select participants by tournament id %v: %w", tournamentKey, err)
		}
		return nil
	})
	eg.Go(func() (err error) {
		matchRows, err = svc.Querier.SelectMatchesByTournamentId(egCtx, pgKey)
		if err != nil {
			return fmt.Errorf("select matches by tournament id %v: %w", tournamentKey, err)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return FullTournamentDTO{}, err
	}

	tournament, err := mapTournamentRow(tournamentRow)
	if err != nil {
		return FullTournamentDTO{}, fmt.Errorf("map tournament from row: %w", err)
	}

	participants := make([]ParticipantDTO, 0, len(participantRows))
	for _, row := range participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	matches := make([]MatchDTO, 0, len(matchRows))
	for _, row := range matchRows {
		match, err := mapTourneyMatchFromRow(row)
		if err != nil {
			return FullTournamentDTO{}, fmt.Errorf("map tournament match from row: %w", err)
		}
		matches = append(matches, match)
	}
	return FullTournamentDTO{TournamentDTO: tournament, Participants: participants, Matches: matches}, nil
}

func mapTourneyParticipantFromRow(participant sqlc.SelectParticipantsByTournamentIdRow) ParticipantDTO {
	return ParticipantDTO{
		TournamentKey: participant.TournamentKey.Bytes,
		JoinedOn:      participant.TournamentJoinedOn.Time,
		LbdUserDTO: mapLbdUser(sqlc.SelectUserWithEloByIDsRow{
			ID:         participant.UserID,
			Username:   participant.Username,
			Country:    participant.Country,
			Bio:        participant.Bio,
			JoinedOn:   participant.UserJoinedOn,
			Elo:        participant.Elo,
			HighestElo: participant.HighestElo,
			Wins:       participant.Wins,
			Losses:     participant.Losses,
			Draws:      participant.Draws,
		}),
	}
}

func mapTourneyMatchFromRow(match sqlc.SelectMatchesByTournamentIdRow) (MatchDTO, error) {
	var tournamentReplay *TournamentReplay
	// invariant: if replayID is non null, all other replay columns will also be non null.
	if match.ReplayID.Valid {
		replayResult, resultErr := enum.Parse(match.Result.ResultEnum, ReplayResultEnums)
		replayCause, causeErr := enum.Parse(match.Cause.CauseEnum, ReplayCauseEnums)
		replayMode, modeErr := enum.Parse(match.Mode.ModeEnum, GameModeEnums)

		if err := errors.Join(resultErr, causeErr, modeErr); err != nil {
			return MatchDTO{}, err
		}

		replay := ReplayDTO{
			ID:          match.ReplayID.Int64,
			WhiteID:     match.WhiteID.Int64,
			BlackID:     match.BlackID.Int64,
			Result:      replayResult,
			Cause:       replayCause,
			Mode:        replayMode,
			WinEloDiff:  match.WinEloDiff.Float64,
			LoseEloDiff: match.LoseEloDiff.Float64,
			PlayedOn:    match.PlayedOn.Time,
		}
		tournamentReplay = ptr(TournamentReplay{
			ReplayDTO:     replay,
			ReplayViewDto: MakeReplayViewDto(replay),
		})
	}
	return MatchDTO{
		ID:            match.TournamentMatchID,
		GameID:        match.GameID.String(), // null gameID will be an empty string.
		TournamentKey: match.TournamentKey.Bytes,
		Depth:         match.Depth,
		CreatedOn:     match.CreatedOn.Time,
		Replay:        tournamentReplay,
	}, nil
}

func mapTournamentRow(tournament sqlc.SelectTournamentByIdRow) (TournamentDTO, error) {
	tournamentStatus, statusErr := enum.Parse(tournament.Status, TournamentStatusEnums)
	gameMode, modeErr := enum.Parse(tournament.Mode, GameModeEnums)

	if err := errors.Join(statusErr, modeErr); err != nil {
		return TournamentDTO{}, err
	}

	return TournamentDTO{
		ID:             tournament.ID,
		TournamentKey:  tournament.TournamentKey.Bytes,
		Name:           tournament.Name,
		Depth:          tournament.Depth,
		MaxPlayerCount: PlayersAtDepth(tournament.Depth),
		ScheduledOn:    tournament.ScheduledOn.Time,
		IsScheduled:    tournament.ScheduledOn.Valid,
		CreatedOn:      tournament.CreatedOn.Time,
		CreatedBy:      tournament.CreatedBy,
		Status:         tournamentStatus,
		Mode:           gameMode,
	}, nil
}

func (svc *Services) GetTournamentByIDWithRanks(ctx context.Context, tournamentKey uuid.UUID) (FullTournamentDTO, error) {
	tournament, err := svc.getTournamentByID(ctx, tournamentKey)
	if err != nil {
		return FullTournamentDTO{}, err
	}
	participantIDs := make([]int64, 0, len(tournament.Participants))
	for _, participant := range tournament.Participants {
		participantIDs = append(participantIDs, participant.ID)
	}

	userLdbRanksMap, err := svc.getUsersLeaderboardRank(ctx, participantIDs, tournament.Mode)
	if err != nil {
		return FullTournamentDTO{}, fmt.Errorf("get users leaderboard rank: %w", err)
	}
	for i := range tournament.Participants {
		tournament.Participants[i].Rank = userLdbRanksMap[tournament.Participants[i].ID]
	}

	slog.InfoContext(ctx, "selected tournament", "tournament", tournament)
	return tournament, nil
}

func (svc *Services) GetTournaments(ctx context.Context, participantID int64, afterID int64, perPage int32) ([]TournamentDTO, error) {
	isParticipantProvided := participantID >= 0
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	var tournaments []TournamentDTO
	var err error

	if isParticipantProvided {
		var tournamentRows []sqlc.SelectTournamentsByParticipantRow
		tournamentRows, err = svc.Querier.SelectTournamentsByParticipant(ctx, sqlc.SelectTournamentsByParticipantParams{
			UserID:  participantID,
			AfterID: afterID,
			PerPage: perPage,
		})
		tournaments, err = mapTournamentRows(tournamentRows, func(t sqlc.SelectTournamentsByParticipantRow) (TournamentDTO, error) {
			return mapTournamentRow(sqlc.SelectTournamentByIdRow(t))
		})
	} else {
		var tournamentRows []sqlc.SelectTournamentsRow
		tournamentRows, err = svc.Querier.SelectTournaments(ctx, sqlc.SelectTournamentsParams{
			AfterID: afterID,
			PerPage: perPage,
		})
		tournaments, err = mapTournamentRows(tournamentRows, func(t sqlc.SelectTournamentsRow) (TournamentDTO, error) {
			return mapTournamentRow(sqlc.SelectTournamentByIdRow(t))
		})
	}
	if err != nil {
		return nil, fmt.Errorf("select tournaments by participantID %d, afterID %d: %w", participantID, afterID, err)
	}

	slog.InfoContext(ctx, "selected tournaments", "tournaments", tournaments)
	return tournaments, nil
}

type tournamentRowType interface {
	sqlc.SelectTournamentsRow | sqlc.SelectTournamentsByParticipantRow
}

func mapTournamentRows[Row tournamentRowType](tournamentRows []Row, fn func(tournament Row) (TournamentDTO, error)) ([]TournamentDTO, error) {
	var tournaments []TournamentDTO
	for _, row := range tournamentRows {
		tournament, err := fn(row)
		if err != nil {
			return nil, fmt.Errorf("map tournament from row: %w", err)
		}
		tournaments = append(tournaments, tournament)
	}
	return tournaments, nil
}

var (
	ErrTournamentNotLobby           = fmt.Errorf("tournament is not in lobby status")
	ErrTooManyParticipants          = fmt.Errorf("tournament is full")
	ErrTournamentAlreadyJoined      = fmt.Errorf("user is already a participant of this tournament")
	ErrInvalidTournamentParticipant = fmt.Errorf("tournament participant is invalid")
)

type JoinTournamentInst struct {
	TournamentKey uuid.UUID
	JoiningUserID int64
	InsertionTime time.Time
}

func (svc *Services) joinTournamentTx(ctx context.Context, inst JoinTournamentInst) error {
	svc.DB.ExecTx(ctx, db.Txn{
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			return joinTournament(ctx, querier, inst)
		},
	})
	return nil
}

func joinTournament(ctx context.Context, querier sqlc.Querier, inst JoinTournamentInst) error {
	tournamentRow, err := querier.SelectTournamentWithParticipantCount(ctx, pgtype.UUID{Bytes: inst.TournamentKey, Valid: true})
	if IsErrNoRows(err) {
		return ErrTournamentNotFound
	} else if err != nil {
		return fmt.Errorf("select tournament %v: %w", inst.TournamentKey, err)
	}

	isNotScheduled := !tournamentRow.ScheduledOn.Valid
	maxPlayerCount := PlayersAtDepth(tournamentRow.Depth)
	participantCount := tournamentRow.ParticipantCount

	status, err := enum.Parse(tournamentRow.Status, TournamentStatusEnums)
	if err != nil {
		return err
	}

	isCapacityReached := func() bool {
		return participantCount >= maxPlayerCount
	}

	if status != TournamentLobby {
		return ErrTournamentNotLobby
	}
	if isCapacityReached() {
		return ErrTooManyParticipants
	}

	if dbErr := querier.InsertTournamentParticipant(ctx, sqlc.InsertTournamentParticipantParams{
		TournamentKey: pgtype.UUID{Bytes: inst.TournamentKey, Valid: true},
		UserID:        inst.JoiningUserID,
		JoinedOn:      pgtype.Timestamptz{Time: inst.InsertionTime, Valid: true},
	}); dbErr != nil {
		if svcErr := mapTournamentInsertErr(dbErr); svcErr != nil {
			return svcErr
		}
		return fmt.Errorf("insert tournament participant %+v: %w", inst, dbErr)
	}
	participantCount++

	if isNotScheduled && isCapacityReached() {
		// if not scheduled and ready to start, queue tournament to be started immedietly
		if err := pushAdvanceTournamentEvent(ctx, querier, inst.TournamentKey); err != nil {
			return err
		}
	}

	slog.InfoContext(ctx, "joined tournament",
		"tournamentKey", inst.TournamentKey, "tournamentRow", tournamentRow, "userID", inst.JoiningUserID)
	return nil
}

func mapTournamentInsertErr(err error) error {
	return mapInsertErr(err, ErrTournamentAlreadyJoined, ErrInvalidTournamentParticipant)
}

type TournamentUpdt struct {
	Status  TournamentStatus
	Matches []CreatedMatchDTO
}

type CreatedMatchDTO struct {
	GameID      uuid.UUID
	GameMode    GameMode
	PlayerOneID int64
	PlayerTwoID int64
}

//func advanceTournament(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, insertionTime uuid.UUID) (TournamentUpdt, error) {
//
//	if len(participantIDs)%2 != 0 {
//		return nil, fmt.Errorf("assertion error: number of partiticipants should be even: %d", len(participantIDs))
//	}
//
//	var matches []CreatedMatchDTO
//	var insts []sqlc.BatchInsertTournamentMatchParams
//
//	for i := 0; i < len(participantIDs); i += 2 {
//		gameID := uuid.New()
//		matches = append(matches, CreatedMatchDTO{
//			GameID:      gameID,
//			GameMode:    gameMode,
//			PlayerOneID: participantIDs[i],
//			PlayerTwoID: participantIDs[i+1],
//		})
//		insts = append(insts, sqlc.BatchInsertTournamentMatchParams{
//			TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
//			GameID:        pgtype.UUID{Bytes: match.GameID, Valid: true},
//			Depth:         1,
//			CreatedOn:     pgtype.Timestamptz{Time: insertionTime, Valid: true},
//		})
//	}
//
//	var batchInsertErrs []error
//	querier.BatchInsertTournamentMatch(ctx, insts).Exec(func(i int, err error) {
//		if err != nil {
//			batchInsertErrs = append(batchInsertErrs, fmt.Errorf("batch %d: inserting tournament matches %+v: %w", i, insts[i], err))
//		}
//	})
//
//	return TournamentUpdt{Status: status, Matches: matches}, errors.Join(batchInsertErrs...)
//}

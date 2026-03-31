package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/internal/enum"
	"log/slog"
	"math"
	"time"
)

type TournamentDTO struct {
	ID             int64            `json:"id"`
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
	TournamentID int64     `json:"tournamentId"`
	JoinedOn     time.Time `json:"joinedOn"`
	LbdUserDTO             // fetches the leaderboard data for the mode the tournament is in.
}
type TournamentReplay struct {
	ReplayDTO
	ReplayViewDto
}

type MatchDTO struct {
	ID           int64  `json:"id"`
	GameID       string `json:"gameId"`
	TournamentID int64  `json:"tournamentId"`
	Depth        int32  `json:"depth"`
	//WhiteID      int64      `json:"whiteId"`
	//BlackID      int64      `json:"blackId"`
	CreatedOn time.Time         `json:"createdOn"`
	Replay    *TournamentReplay `json:"replay"`
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
	TournamentKey string        `json:"tournamentKey"`
	Name          string        `json:"name"`
	Depth         int32         `json:"depth" json:"depth"`
	Mode          GameMode      `json:"mode" json:"mode"`
	ScheduledIn   time.Duration `json:"scheduledIn"`
	CreatedOn     time.Time     `json:"createdOn"`
	CreatedBy     int64         `json:"createdBy"`
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
		TournamentKey: inst.TournamentKey,
		Name:          inst.Name,
		Depth:         inst.Depth,
		Status:        sqlc.TournamentStatusEnum(InsertionStatus),
		ScheduledOn:   pgtype.Timestamptz{Time: scheduledOn, Valid: isScheduledTournament},
		CreatedOn:     pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		CreatedBy:     inst.CreatedBy,
		UpdatedOn:     pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		Mode:          sqlc.ModeEnum(inst.Mode.String()),
	})
	if err != nil {
		return 0, fmt.Errorf("insert tournament: %w", err)
	}

	slog.InfoContext(ctx, "created tournament", "tournamentID", tournamentID)
	return tournamentID, nil
}

func (svc *Services) JoinTournament(ctx context.Context, userID int64) error {
	return nil
}

var ErrNoTournament = fmt.Errorf("tournament does not exist")

func (svc *Services) getTournamentByID(ctx context.Context, tournamentID int64) (FullTournamentDTO, error) {
	var tournamentRow sqlc.SelectTournamentByIdRow
	var matchRows []sqlc.SelectMatchesByTournamentIdRow
	var participantRows []sqlc.SelectParticipantsByTournamentIdRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		tournamentRow, err = svc.Querier.SelectTournamentById(egCtx, tournamentID)
		if IsErrNoRows(err) {
			return ErrNoTournament
		} else if err != nil {
			return fmt.Errorf("select tournament by id %d: %w", tournamentID, err)
		}
		return nil
	})
	eg.Go(func() (err error) {
		participantRows, err = svc.Querier.SelectParticipantsByTournamentId(egCtx, tournamentID)
		if err != nil {
			return fmt.Errorf("select participants by tournament id %d: %w", tournamentID, err)
		}
		return nil
	})
	eg.Go(func() (err error) {
		matchRows, err = svc.Querier.SelectMatchesByTournamentId(egCtx, tournamentID)
		if err != nil {
			return fmt.Errorf("select matches by tournament id %d: %w", tournamentID, err)
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

func (svc *Services) GetTournamentByIDWithRanks(ctx context.Context, tournamentID int64) (FullTournamentDTO, error) {
	tournament, err := svc.getTournamentByID(ctx, tournamentID)
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

func (svc *Services) advanceTournamentTx(ctx context.Context, tournamentID int64) error {
	return nil
}

func mapTournamentRow(tournament sqlc.SelectTournamentByIdRow) (TournamentDTO, error) {
	tournamentStatus, statusErr := enum.Parse(tournament.Status, TournamentStatusMembers)
	gameMode, modeErr := enum.Parse(tournament.Mode, GameModeMembers)

	if err := errors.Join(statusErr, modeErr); err != nil {
		return TournamentDTO{}, err
	}

	return TournamentDTO{
		ID:             tournament.ID,
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

type ManyTournamentRow interface {
	sqlc.SelectTournamentsRow | sqlc.SelectTournamentsByParticipantRow
}

func mapTournamentRows[Row ManyTournamentRow](tournamentRows []Row, fn func(tournament Row) (TournamentDTO, error)) ([]TournamentDTO, error) {
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

func mapTourneyParticipantFromRow(participant sqlc.SelectParticipantsByTournamentIdRow) ParticipantDTO {
	return ParticipantDTO{
		TournamentID: participant.TournamentID,
		JoinedOn:     participant.TournamentJoinedOn.Time,
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
		replayResult, resultErr := enum.Parse(match.Result.ResultEnum, ReplayResultMembers)
		replayCause, causeErr := enum.Parse(match.Cause.CauseEnum, ReplayCauseMembers)
		replayMode, modeErr := enum.Parse(match.Mode.ModeEnum, GameModeMembers)

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
		ID:           match.TournamentMatchID,
		GameID:       match.GameID.String, // null gameID will be an empty string.
		TournamentID: match.TournamentID,
		Depth:        match.Depth,
		CreatedOn:    match.CreatedOn.Time,
		Replay:       tournamentReplay,
	}, nil
}

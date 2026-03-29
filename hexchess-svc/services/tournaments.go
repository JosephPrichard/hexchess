package svc

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db/sqlc"
	"log/slog"
	"math"
	"time"
)

type TournamentDTO struct {
	ID             int64            `json:"id"`
	Depth          int32            `json:"depth"`
	MaxPlayerCount int32            `json:"maxPlayerCount"`
	ScheduledOn    time.Time        `json:"scheduledOn"`
	IsScheduled    bool             `json:"isScheduled"`
	CreatedOn      time.Time        `json:"createdOn"`
	Status         TournamentStatus `json:"status"`
	Mode           GameMode         `json:"mode"`
}

type TournamentParticipantDTO struct {
	TournamentID int64     `json:"tournamentId"`
	JoinedOn     time.Time `json:"joinedOn"`
	LbdUserDTO             // fetches the leaderboard data for the mode the tournament is in.
}

type TournamentReplay struct {
	ID           int64      `json:"id"`
	GameID       string     `json:"gameId"`
	TournamentID int64      `json:"tournamentId"`
	Depth        int32      `json:"depth"`
	WhiteID      int64      `json:"whiteId"`
	BlackID      int64      `json:"blackId"`
	CreatedOn    time.Time  `json:"createdOn"`
	Replay       *ReplayDTO `json:"replay"`
}

type TournamentMatchDTO struct {
	ID           int64  `json:"id"`
	GameID       string `json:"gameId"`
	TournamentID int64  `json:"tournamentId"`
	Depth        int32  `json:"depth"`
	//WhiteID      int64      `json:"whiteId"`
	//BlackID      int64      `json:"blackId"`
	CreatedOn time.Time  `json:"createdOn"`
	Replay    *ReplayDTO `json:"replay"`
}

type FullTournamentDTO struct {
	Participants []TournamentParticipantDTO `json:"participants"`
	Matches      []TournamentMatchDTO       `json:"matches"`
	TournamentDTO
}

const MaxTournamentDepth = 5 // equivalent to 32 players, 16 matches first round, 31 matches in total, 5 matches per player

func PlayersAtDepth(depth int32) int32 {
	return int32(math.Pow(2, float64(depth)))
}

func MatchesAtDepth(depth int32) int32 {
	return PlayersAtDepth(depth) / 2
}

func mapTournamentFromRow(tournament sqlc.Tournament) (TournamentDTO, error) {
	status, err := ParseTournamentStatus(tournament.Status)
	if err != nil {
		return TournamentDTO{}, err
	}
	mode, err := ParseGameMode(tournament.Mode)
	if err != nil {
		return TournamentDTO{}, err
	}

	return TournamentDTO{
		ID:             tournament.ID,
		Depth:          tournament.Depth,
		MaxPlayerCount: PlayersAtDepth(tournament.Depth),
		ScheduledOn:    tournament.ScheduledOn.Time,
		IsScheduled:    tournament.ScheduledOn.Valid,
		CreatedOn:      tournament.CreatedOn.Time,
		Status:         status,
		Mode:           mode,
	}, nil
}

func mapTourneyParticipantFromRow(participant sqlc.SelectParticipantsByTournamentIdRow) TournamentParticipantDTO {
	return TournamentParticipantDTO{
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

func mapTourneyMatchFromRow(match sqlc.SelectMatchesByTournamentIdRow) (TournamentMatchDTO, error) {
	result, err := ParseReplayResult(match.Result.ResultEnum)
	if err != nil {
		return TournamentMatchDTO{}, err
	}
	cause, err := ParseReplayCause(match.Cause.CauseEnum)
	if err != nil {
		return TournamentMatchDTO{}, err
	}
	mode, err := ParseGameMode(match.Mode.ModeEnum)
	if err != nil {
		return TournamentMatchDTO{}, err
	}

	var replay *ReplayDTO
	if match.ReplayID.Valid {
		// invariant: if replayID is non null, all other replay columns will also be non null.
		replay = ptr(ReplayDTO{
			ID:          match.ReplayID.Int64,
			WhiteID:     match.WhiteID.Int64,
			BlackID:     match.BlackID.Int64,
			Result:      result,
			Cause:       cause,
			Mode:        mode,
			WinEloDiff:  match.WinEloDiff.Float64,
			LoseEloDiff: match.LoseEloDiff.Float64,
			PlayedOn:    match.PlayedOn.Time,
		}.Compute())
	}

	return TournamentMatchDTO{
		ID:           match.TournamentMatchID,
		GameID:       match.GameID.String, // null gameID will be an empty string.
		TournamentID: match.TournamentID,
		Depth:        match.Depth,
		CreatedOn:    match.CreatedOn.Time,
		Replay:       replay,
	}, nil
}

type InvalidDepthError struct {
	actualDepth int32
}

func (e InvalidDepthError) Error() string {
	return fmt.Sprintf("invalid depth: %d, must be less than %d", e.actualDepth, MaxTournamentDepth)
}

type TournamentInst struct {
	Depth       int32     `json:"depth"`
	Mode        GameMode  `json:"mode"`
	ScheduledOn time.Time `json:"scheduledOn"`
	CreatedOn   time.Time
}

func (svc *Services) CreateTournament(ctx context.Context, inst TournamentInst) (int64, error) {
	if inst.Depth > MaxTournamentDepth {
		return 0, InvalidDepthError{actualDepth: inst.Depth}
	}

	isScheduledTournament := !inst.ScheduledOn.IsZero()
	if inst.CreatedOn.IsZero() {
		inst.CreatedOn = time.Now()
	}

	status := TournamentLobby

	tournamentID, err := svc.Querier.InsertTournament(ctx, sqlc.InsertTournamentParams{
		Depth:       inst.Depth,
		Status:      sqlc.TournamentStatusEnum(status.String()),
		ScheduledOn: pgtype.Timestamptz{Time: inst.ScheduledOn, Valid: isScheduledTournament},
		CreatedOn:   pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		UpdatedOn:   pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		Mode:        sqlc.ModeEnum(inst.Mode.String()),
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

func (svc *Services) GetTournamentByID(ctx context.Context, tournamentID int64) (FullTournamentDTO, error) {
	eg, egCtx := errgroup.WithContext(ctx)

	var tournamentRow sqlc.Tournament
	var matchRows []sqlc.SelectMatchesByTournamentIdRow
	var participantRows []sqlc.SelectParticipantsByTournamentIdRow

	eg.Go(func() (err error) {
		tournamentRow, err = svc.Querier.SelectTournamentById(egCtx, tournamentID)
		if err != nil {
			return fmt.Errorf("select tournament by id: %w", err)
		}
		return nil
	})
	eg.Go(func() (err error) {
		participantRows, err = svc.Querier.SelectParticipantsByTournamentId(egCtx, tournamentID)
		if err != nil {
			return fmt.Errorf("select participants by tournament id: %w", err)
		}
		return nil
	})
	eg.Go(func() (err error) {
		matchRows, err = svc.Querier.SelectMatchesByTournamentId(egCtx, tournamentID)
		if err != nil {
			return fmt.Errorf("select matches by tournament id: %w", err)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return FullTournamentDTO{}, err
	}

	tournament, err := mapTournamentFromRow(tournamentRow)
	if err != nil {
		return FullTournamentDTO{}, err
	}

	participants := make([]TournamentParticipantDTO, 0, len(participantRows))
	for _, row := range participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	matches := make([]TournamentMatchDTO, 0, len(matchRows))
	for _, row := range matchRows {
		match, err := mapTourneyMatchFromRow(row)
		if err != nil {
			return FullTournamentDTO{}, fmt.Errorf("map tourney match from row: %w", err)
		}
		matches = append(matches, match)
	}

	userLdbRanksMap, err := svc.getUsersLeaderboardRank(ctx, nil, tournament.Mode)
	if err != nil {
		return FullTournamentDTO{}, fmt.Errorf("get users leaderboard rank: %w", err)
	}
	for _, participant := range participants {
		participant.Rank = userLdbRanksMap[participant.ID]
	}

	slog.InfoContext(ctx, "selected tournament", "tournament", tournament, "participants", participants, "matches", matches)

	return FullTournamentDTO{
		TournamentDTO: tournament,
		Participants:  participants,
		Matches:       matches,
	}, nil
}

func (svc *Services) GetTournaments(ctx context.Context, participantID int64, afterID int64, perPage int32) ([]TournamentDTO, error) {
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	var tournamentRows []sqlc.Tournament
	var err error
	if participantID >= 0 {
		tournamentRows, err = svc.Querier.SelectTournamentsByParticipant(ctx, sqlc.SelectTournamentsByParticipantParams{
			UserID:  participantID,
			AfterID: afterID,
			PerPage: perPage,
		})
	} else {
		tournamentRows, err = svc.Querier.SelectTournaments(ctx, sqlc.SelectTournamentsParams{
			AfterID: afterID,
			PerPage: perPage,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("select tournaments by participantID %d, afterID %d: %w", participantID, afterID, err)
	}

	tournaments := make([]TournamentDTO, 0, len(tournamentRows))
	for _, row := range tournamentRows {
		tournament, err := mapTournamentFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("map tournament from row: %w", err)
		}
		tournaments = append(tournaments, tournament)
	}
	slog.InfoContext(ctx, "selected tournaments", "tournaments", tournaments)

	return tournaments, nil
}

func (svc *Services) advanceTournamentTx(ctx context.Context, tournamentID int64) error {
	return nil
}

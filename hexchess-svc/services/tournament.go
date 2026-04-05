package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/internal/enum"
	"hexchess-svc/internal/tree"
	"log/slog"
	"math"
	"math/rand"
	"time"
)

type TournamentDTO struct {
	ID             int64             `json:"ID"`
	TournamentKey  uuid.UUID         `json:"tournamentKey"`
	Name           string            `json:"name"`
	Rounds         int32             `json:"rounds"`
	MaxPlayerCount int32             `json:"maxPlayerCount"`
	ScheduledOn    time.Time         `json:"scheduledOn"`
	IsScheduled    bool              `json:"isScheduled"`
	CreatedOn      time.Time         `json:"createdOn"`
	CreatedBy      int64             `json:"createdBy"`
	Ruleset        TournamentRuleset `json:"ruleset"`
	Status         TournamentStatus  `json:"status"`
	Mode           GameMode          `json:"mode"`
}

type ParticipantDTO struct {
	TournamentKey uuid.UUID `json:"tournamentKey"`
	JoinedOn      time.Time `json:"joinedOn"`
	LbdUserDTO              // fetches the leaderboard data for the mode the tournament is in.
}
type TournamentReplay struct {
	ReplayDTO
	ReplayViewDTO
}

type MatchDTO struct {
	ID            int64             `json:"ID"`
	GameID        string            `json:"gameID"`
	TournamentKey uuid.UUID         `json:"tournamentKey"`
	Round         int32             `json:"round"`
	CreatedOn     time.Time         `json:"createdOn"`
	Replay        *TournamentReplay `json:"replay"`
}

type FullTournamentDTO struct {
	Participants []ParticipantDTO `json:"participants"`
	Matches      []MatchDTO       `json:"matches"`
	TournamentDTO
}

const MaxTournamentDepth = 5 // equivalent to 32 players, 16 matches first round, 31 matches in total, 5 matches per player

type InvalidDepthError struct {
	actualDepth int32
}

func (e InvalidDepthError) Error() string {
	return fmt.Sprintf("invalid depth: %d, must be less than %d and larger than 0", e.actualDepth, MaxTournamentDepth)
}

var ErrTournamentNotFound = fmt.Errorf("tournament does not exist")

func mapTourneyParticipantFromRow(participant sqlc.SelectParticipantsByTournamentIDRow) ParticipantDTO {
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

func mapTourneyMatchFromRow(match sqlc.SelectReplayMatchesByTournamentIDRow) (MatchDTO, error) {
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
			ReplayViewDTO: MakeReplayViewDTO(replay),
		})
	}
	return MatchDTO{
		GameID:        match.GameID, // null gameID will be an empty string.
		TournamentKey: match.TournamentKey.Bytes,
		Round:         match.Round,
		CreatedOn:     match.CreatedOn.Time,
		Replay:        tournamentReplay,
	}, nil
}

func mapTournamentByIdRow(tournament sqlc.SelectTournamentByIDRow) (TournamentDTO, error) {
	tournamentStatus, statusErr := enum.Parse(tournament.Status, TournamentStatusEnums)
	gameMode, modeErr := enum.Parse(tournament.Mode, GameModeEnums)

	if err := errors.Join(statusErr, modeErr); err != nil {
		return TournamentDTO{}, err
	}

	return TournamentDTO{
		ID:             tournament.ID,
		TournamentKey:  tournament.TournamentKey.Bytes,
		Name:           tournament.Name,
		Rounds:         tournament.Rounds,
		MaxPlayerCount: int32(tree.ElementsAtFirstDepth(int(tournament.Rounds))),
		ScheduledOn:    tournament.ScheduledOn.Time,
		IsScheduled:    tournament.ScheduledOn.Valid,
		CreatedOn:      tournament.CreatedOn.Time,
		CreatedBy:      tournament.CreatedBy,
		Status:         tournamentStatus,
		Mode:           gameMode,
	}, nil
}

func (svc *Services) GetFullTournamentByID(ctx context.Context, tournamentKey uuid.UUID) (t FullTournamentDTO, err error) {
	var tournamentRow sqlc.SelectTournamentByIDRow
	var matchRows []sqlc.SelectReplayMatchesByTournamentIDRow
	var participantRows []sqlc.SelectParticipantsByTournamentIDRow

	eg, egCtx := errgroup.WithContext(ctx)

	pgTournamentKey := pgtype.UUID{Bytes: tournamentKey, Valid: true}

	eg.Go(func() (err error) {
		tournamentRow, err = svc.Querier.SelectTournamentByID(egCtx, pgTournamentKey)
		if IsErrNoRows(err) {
			return ErrTournamentNotFound
		} else if err != nil {
			return fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
		}
		return nil
	})
	eg.Go(func() (err error) {
		participantRows, err = svc.Querier.SelectParticipantsByTournamentID(egCtx, pgTournamentKey)
		if err != nil {
			return fmt.Errorf("select participants by tournament key %v: %w", tournamentKey, err)
		}
		return nil
	})
	eg.Go(func() (err error) {
		matchRows, err = svc.Querier.SelectReplayMatchesByTournamentID(egCtx, pgTournamentKey)
		if err != nil {
			return fmt.Errorf("select matches by tournament key %v: %w", tournamentKey, err)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return t, err
	}

	slog.InfoContext(ctx, "selected tournament", "tournament", tournamentRow, "matchRows", matchRows, "participantRows", participantRows)

	fullTournament, err := mapFullTournament(mapFullTournamentArgs{
		tournamentRow:   tournamentRow,
		matchRows:       matchRows,
		participantRows: participantRows,
	})
	if err != nil {
		return t, err
	}
	participantIDs := make([]int64, 0, len(fullTournament.Participants))
	for _, participant := range fullTournament.Participants {
		participantIDs = append(participantIDs, participant.ID)
	}

	userLdbRanksMap, err := svc.getUsersLeaderboardRank(ctx, participantIDs, fullTournament.Mode)
	if err != nil {
		return t, fmt.Errorf("get users %+v leaderboard rank: %w", participantIDs, err)
	}
	for i := range fullTournament.Participants {
		fullTournament.Participants[i].Rank = userLdbRanksMap[fullTournament.Participants[i].ID]
	}

	slog.InfoContext(ctx, "retrieved full tournament", "fullTournament", fullTournament)
	return fullTournament, nil
}

type mapFullTournamentArgs struct {
	tournamentRow   sqlc.SelectTournamentByIDRow
	matchRows       []sqlc.SelectReplayMatchesByTournamentIDRow
	participantRows []sqlc.SelectParticipantsByTournamentIDRow
}

func mapFullTournament(args mapFullTournamentArgs) (FullTournamentDTO, error) {
	var tournament TournamentDTO
	participants := make([]ParticipantDTO, 0, len(args.participantRows))
	matches := make([]MatchDTO, 0, len(args.matchRows))

	var mapErrors []error

	tournament, err := mapTournamentByIdRow(args.tournamentRow)
	if err != nil {
		mapErrors = append(mapErrors, fmt.Errorf("map tournament row: %w", err))
	}

	for _, row := range args.participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	for i, row := range args.matchRows {
		match, err := mapTourneyMatchFromRow(row)
		if err != nil {
			mapErrors = append(mapErrors, fmt.Errorf("map match %d row: %w", i, err))
			continue
		}
		matches = append(matches, match)
	}

	return FullTournamentDTO{TournamentDTO: tournament, Participants: participants, Matches: matches}, nil
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
		tournaments, err = mapTournamentRows(tournamentRows, mapParticipantTournamentRow)
	} else {
		var tournamentRows []sqlc.SelectTournamentsRow
		tournamentRows, err = svc.Querier.SelectTournaments(ctx, sqlc.SelectTournamentsParams{
			AfterID: afterID,
			PerPage: perPage,
		})
		tournaments, err = mapTournamentRows(tournamentRows, mapTournamentRow)
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

func mapParticipantTournamentRow(t sqlc.SelectTournamentsByParticipantRow) (TournamentDTO, error) {
	return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
}

func mapTournamentRow(t sqlc.SelectTournamentsRow) (TournamentDTO, error) {
	return mapTournamentByIdRow(sqlc.SelectTournamentByIDRow(t))
}

func mapTournamentRows[Row tournamentRowType](tournamentRows []Row, fn func(tournament Row) (TournamentDTO, error)) ([]TournamentDTO, error) {
	var tournaments []TournamentDTO
	for _, row := range tournamentRows {
		tournament, err := fn(row)
		if err != nil {
			return nil, err
		}
		tournaments = append(tournaments, tournament)
	}
	return tournaments, nil
}

type TournamentInst struct {
	Key         uuid.UUID         `json:"key"`
	Name        string            `json:"name"`
	Rounds      int32             `json:"rounds"`
	Mode        GameMode          `json:"mode"`
	Ruleset     TournamentRuleset `json:"ruleset"`
	ScheduledIn time.Duration     `json:"scheduledIn"`
	CreatedOn   time.Time         `json:"createdOn"`
	CreatedBy   int64             `json:"createdBy"`
}

var InsertionStatus = TournamentLobby.String()

func (svc *Services) CreateTournament(ctx context.Context, inst TournamentInst) (int64, error) {
	if inst.Rounds < 1 || inst.Rounds > MaxTournamentDepth {
		return 0, InvalidDepthError{actualDepth: inst.Rounds}
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
		Rounds:      inst.Rounds,
		Status:      sqlc.TournamentStatusEnum(InsertionStatus),
		ScheduledOn: pgtype.Timestamptz{Time: scheduledOn, Valid: isScheduledTournament},
		CreatedOn:   pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		CreatedBy:   inst.CreatedBy,
		UpdatedOn:   pgtype.Timestamptz{Time: inst.CreatedOn, Valid: true},
		Mode:        sqlc.ModeEnum(inst.Mode.String()),
		Ruleset:     sqlc.TournamentRulesetEnum(inst.Ruleset.String()),
	})
	if err != nil {
		return 0, fmt.Errorf("insert tournament: %w", err)
	}

	slog.InfoContext(ctx, "created tournament", "tournamentKey", tournamentID)
	return tournamentID, nil
}

func (svc *Services) LeaveTournament(ctx context.Context, tournamentKey uuid.UUID, userID int64) (bool, error) {
	deletedIDs, err := svc.Querier.DeleteTournamentParticipant(ctx, sqlc.DeleteTournamentParticipantParams{
		TournamentKey: pgtype.UUID{Bytes: tournamentKey, Valid: true},
		UserID:        userID,
	})
	if err != nil {
		return false, fmt.Errorf("delete participant %d from tournament %s: %w", userID, tournamentKey, err)
	}

	slog.InfoContext(ctx, "deleted tournament participant", "tournamentKey", tournamentKey, "deletedIDs", deletedIDs)
	return len(deletedIDs) > 0, nil
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

func (svc *Services) JoinTournamentTx(ctx context.Context, inst JoinTournamentInst) error {
	return svc.DB.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 reads status S1 and participant count P1, then inserts participants to create new participant count P2
		// Between reading P1 and P2, another query inserts a participant to create P3
		// P2 will be appended onto P3 rather than P1, even though the validation was run against P1
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			return joinTournament(ctx, querier, inst)
		},
		RetryCount: 3,
	})
}

func joinTournament(ctx context.Context, querier sqlc.Querier, inst JoinTournamentInst) error {
	tournamentRow, err := querier.SelectTournamentWithParticipantCountByID(ctx, pgtype.UUID{Bytes: inst.TournamentKey, Valid: true})
	if IsErrNoRows(err) {
		return ErrTournamentNotFound
	} else if err != nil {
		return fmt.Errorf("select tournament %v: %w", inst.TournamentKey, err)
	}

	status, statusErr := enum.Parse(tournamentRow.Status, TournamentStatusEnums)
	ruleset, rulesetErr := enum.Parse(tournamentRow.Ruleset, TournamentRulesetEnums)
	if err := errors.Join(statusErr, rulesetErr); err != nil {
		return err
	}

	if status != TournamentLobby {
		return ErrTournamentNotLobby
	}

	if ruleset == TournamentKnockout {
		// knockout rulesets use the `Rounds` field to decide the maximum number of players
		maxKnckoutPlayerCount := int32(tree.ElementsAtFirstDepth(int(tournamentRow.Rounds)))
		isCapacityReached := tournamentRow.ParticipantCount >= maxKnckoutPlayerCount
		if isCapacityReached {
			return ErrTooManyParticipants
		}
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

	slog.InfoContext(ctx, "joined tournament",
		"tournamentKey", inst.TournamentKey, "tournamentRow", tournamentRow, "userID", inst.JoiningUserID)
	return nil
}

func mapTournamentInsertErr(err error) error {
	return mapInsertErr(err, ErrTournamentAlreadyJoined, ErrInvalidTournamentParticipant)
}

type AdvanceTournamentMatchDTO struct {
	GameID      string
	GameMode    GameMode
	PlayerOneID int64
	PlayerTwoID int64
}

var (
	ErrInvalidStartTournamentStatus = fmt.Errorf("tournament must be in LOBBY status to start")
	ErrInvalidParticipantCount      = fmt.Errorf("tournament does not have enough participants create matches")
	ErrInvalidParticipantParity     = fmt.Errorf("participant count must be even")
)

func (svc *Services) StartTournamentTx(ctx context.Context, tournamentKey uuid.UUID) ([]AdvanceTournamentMatchDTO, error) {
	var matches []AdvanceTournamentMatchDTO

	err := svc.DB.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Write Skew):
		// T1 selects participationIDs P1 and creates and inserts matches M1
		// Between reading of P1 and insertion of M1, another query deletes a participant to create partcipationID state P2
		// M1 has created and returned games with regards to P1 and may contain matches with players not contained in P2
		// Case 2 (Lost Update):
		// T1 selects the status S1 and uses it to decide that matches M1 can be created, and S2 status should be updated
		// Between reading S1 and insertion of M1, another transaction progresses the state to S3 (such as CANCELLED)
		// Status will be overwritten with the new IN_PROGRESS status (S2), S3 is lost
		// This is because only certain status transitions are legal, progression is linear / forward moving
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) (err error) {
			matches, err = startTournament(ctx, querier, tournamentKey, svc.EntropySource.GetNow)
			return
		},
		RetryCount: 3,
	})

	return matches, err
}

func startTournament(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, getInsertionTime func() time.Time) ([]AdvanceTournamentMatchDTO, error) {
	pgTournamentKey := pgtype.UUID{Bytes: tournamentKey, Valid: true}

	tournamentRow, err := querier.SelectTournamentByID(ctx, pgTournamentKey)
	if err != nil {
		return nil, fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
	}
	participantIDs, err := querier.SelectParticipantIDsByTournamentID(ctx, pgTournamentKey)
	if err != nil {
		return nil, fmt.Errorf("select participant ids by tournament key %s: %w", tournamentKey, err)
	}

	status, statusErr := enum.Parse(tournamentRow.Status, TournamentStatusEnums)
	mode, modeErr := enum.Parse(tournamentRow.Mode, GameModeEnums)
	ruleset, rulesetErr := enum.Parse(tournamentRow.Ruleset, TournamentRulesetEnums)
	if err := errors.Join(statusErr, modeErr, rulesetErr); err != nil {
		return nil, err
	}

	if status != TournamentLobby {
		return nil, ErrInvalidStartTournamentStatus
	}

	var matches []AdvanceTournamentMatchDTO

	switch ruleset {
	case TournamentKnockout:
		// validation: 'Knockout' matches are devided by two each time, we need to start the expected power of 2
		if len(participantIDs) != tree.ElementsAtFirstDepth(int(tournamentRow.Rounds)) {
			return nil, ErrInvalidParticipantCount
		}
		matches = makeFirstMatches(participantIDs, mode, DeterministicMatchmaking)
		// invariant: we should have as many matches expected at the first depth
		if len(matches) == tree.NodesAtDepth(int(tournamentRow.Rounds), 1) {
			return nil, MakeMatchStateError(tournamentKey, ErrMatchRoundCount)
		}
	case TournamentSwiss, TournamentRoundRobin:
		// validation: as long as we can match each player with another player, we can start the tournament
		if len(participantIDs)%2 != 0 {
			return nil, ErrInvalidParticipantParity
		}
		matches = makeFirstMatches(participantIDs, mode, DeterministicMatchmaking)
	default:
		return nil, fmt.Errorf("unknown tournament ruleset %s", ruleset)
	}

	if err := putTournamentMatches(ctx, querier, matchmakingInst{
		tournamentKey:        tournamentKey,
		nextTournamentStatus: TournamentInProgress,
		round:                1,
		getInsertionTime:     getInsertionTime,
		matches:              matches,
	}); err != nil {
		return nil, fmt.Errorf("insert tournament %s matches: %w", tournamentKey, err)
	}

	slog.InfoContext(ctx, "started tournament", "tournamentKey", tournamentKey, "matches", matches)
	return matches, nil
}

type FirstRoundMatchmakingKind int

const (
	DeterministicMatchmaking FirstRoundMatchmakingKind = iota
	RandomMatchmaking
)

func makeFirstMatches(participantIDs []int64, gameMode GameMode, matchmaking FirstRoundMatchmakingKind) []AdvanceTournamentMatchDTO {
	// invariant: participant count is always even (`ElementsAtFirstDepth` always returns even)
	if len(participantIDs)%2 == 0 {
		// assert rather than return an error because this property is statically encoded into the `ElementsAtFirstDepth` algorithm
		panic(fmt.Sprintf("participant count %+v is not even", participantIDs))
	}
	var matches []AdvanceTournamentMatchDTO
	switch matchmaking {
	case DeterministicMatchmaking:
	case RandomMatchmaking:
		rand.Shuffle(len(participantIDs), func(i, j int) { participantIDs[i], participantIDs[j] = participantIDs[j], participantIDs[i] })
	}
	for i := 0; i+1 < len(participantIDs); i += 2 {
		gameID := MakeGameID()
		matches = append(matches, AdvanceTournamentMatchDTO{
			GameID:      gameID,
			GameMode:    gameMode,
			PlayerOneID: participantIDs[i],
			PlayerTwoID: participantIDs[i+1],
		})
	}
	return matches
}

var ErrInvalidAdvanceTournamentStatus = errors.New("tournament must be in IN_PROGRESS status to start")

func (svc *Services) AdvanceTournamentTx(ctx context.Context, tournamentKey uuid.UUID) ([]AdvanceTournamentMatchDTO, error) {
	var matches []AdvanceTournamentMatchDTO

	err := svc.DB.ExecTx(ctx, db.Tx{
		// Serializable is required to prevent the following race conditions
		// ditto from `StartTournamentTx`, same cases apply here
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) (err error) {
			matches, err = advanceTournament(ctx, querier, tournamentKey, svc.EntropySource.GetNow)
			return
		},
		RetryCount: 3,
	})

	return matches, err
}

var ErrEmptyMatchesTournament = errors.New("tournament has no matches")
var ErrMatchRoundCount = errors.New("tournament has an invalid finished matches count in round")
var ErrRoundCoherency = errors.New("round should be same across multiple selected matches")

type MatchStateError struct {
	TournamentKey uuid.UUID
	Err           error
}

func MakeMatchStateError(tournamentKey uuid.UUID, err error) MatchStateError {
	return MatchStateError{TournamentKey: tournamentKey, Err: err}
}

func (e MatchStateError) Error() string {
	return fmt.Sprintf("tournament %s state is invalid: %v", e.TournamentKey, e.Err)
}

func advanceTournament(ctx context.Context, querier sqlc.Querier, tournamentKey uuid.UUID, getInsertionTime func() time.Time) ([]AdvanceTournamentMatchDTO, error) {
	pgTournamentKey := pgtype.UUID{Bytes: tournamentKey, Valid: true}

	tournamentRow, err := querier.SelectTournamentByID(ctx, pgTournamentKey)
	if err != nil {
		return nil, fmt.Errorf("select tournament by key %v: %w", tournamentKey, err)
	}
	matchRows, err := querier.SelectLastMatchesByTournamentID(ctx, pgTournamentKey)
	if err != nil {
		return nil, fmt.Errorf("select participant ids by tournament key %s: %w", tournamentKey, err)
	}

	lastCompletedMatches, err := collecCompletedMatches(matchRows)
	if err != nil {
		return nil, MakeMatchStateError(tournamentKey, err)
	}

	status, statusErr := enum.Parse(tournamentRow.Status, TournamentStatusEnums)
	mode, modeErr := enum.Parse(tournamentRow.Mode, GameModeEnums)
	ruleset, rulesetErr := enum.Parse(tournamentRow.Ruleset, TournamentRulesetEnums)
	if err := errors.Join(statusErr, modeErr, rulesetErr); err != nil {
		return nil, MakeMatchStateError(tournamentKey, err)
	}

	// invariant: a tournament should not have been advance if it is not already in progress
	if status != TournamentInProgress {
		return nil, MakeMatchStateError(tournamentKey, ErrInvalidAdvanceTournamentStatus)
	}
	// invariant: a tournament must have any completed matches to be advanced
	if len(lastCompletedMatches) == 0 {
		return nil, MakeMatchStateError(tournamentKey, ErrEmptyMatchesTournament)
	}
	// invariant: all matches have the same round (select query selects by largest/last round)
	lastMatchRound := lastCompletedMatches[0].Round
	for _, row := range lastCompletedMatches {
		if row.Round != lastMatchRound {
			return nil, MakeMatchStateError(tournamentKey, ErrRoundCoherency)
		}
	}

	var nextMatches []AdvanceTournamentMatchDTO
	var nextStatus TournamentStatus
	nextMatchRound := lastMatchRound + 1

	switch ruleset {
	case TournamentKnockout:
		// validation: `Knockout` the last batch of matches are at a completed state
		if len(lastCompletedMatches) == tree.NodesAtDepth(int(tournamentRow.Rounds), int(lastMatchRound)) {
			// additionally, this check makes this operation idempotent within a very short timeframe
			// if two advanceTournament operations run serially, the second one will produce this error
			return nil, MakeMatchStateError(tournamentKey, ErrMatchRoundCount)
		}
		if nextMatchRound == tournamentRow.Rounds {
			nextStatus = TournamentFinished
		}
		if nextStatus != TournamentFinished {
			nextMatches = makeNextMatchesElimination(lastCompletedMatches, mode)
		}
	case TournamentSwiss:
	case TournamentRoundRobin:
	default:
		return nil, fmt.Errorf("unknown tournament ruleset %s", ruleset)
	}

	if err := putTournamentMatches(ctx, querier, matchmakingInst{
		tournamentKey:        tournamentKey,
		nextTournamentStatus: nextStatus,
		round:                nextMatchRound,
		getInsertionTime:     getInsertionTime,
		matches:              nextMatches,
	}); err != nil {
		return nil, fmt.Errorf("insert tournament %s nextMatches: %w", tournamentKey, err)
	}

	slog.InfoContext(ctx, "advanced tournament", "tournamentKey", tournamentKey,
		"nextMatchRound", nextMatchRound, "nextStatus", nextStatus, "nextMatches", nextMatches, "nextMatches", nextMatches)
	return nextMatches, nil
}

type LastMatchDTO struct {
	Round   int32
	WhiteID int64
	BlackID int64
	Result  ReplayResult
}

func collecCompletedMatches(matchRows []sqlc.SelectLastMatchesByTournamentIDRow) ([]LastMatchDTO, error) {
	var matches []LastMatchDTO
	var parseErrs []error

	for _, row := range matchRows {
		// invariant: if row.Result is non-null, so are all other columns
		if !row.Result.Valid {
			break
		}
		result, err := enum.Parse(row.Result.ResultEnum, ReplayResultEnums)
		if err != nil {
			parseErrs = append(parseErrs, err)
			continue
		}
		matches = append(matches, LastMatchDTO{
			Round:   row.Round,
			WhiteID: row.WhiteID.Int64,
			BlackID: row.BlackID.Int64,
			Result:  result,
		})
	}

	return matches, errors.Join(parseErrs...)
}

func tieBreaker(match LastMatchDTO) int64 {
	return match.WhiteID
}

func makeNextMatchesElimination(matches []LastMatchDTO, gameMode GameMode) []AdvanceTournamentMatchDTO {
	// invariant: match count is always even (`NodesAtDepth` always returns even)
	if len(matches)%2 == 0 {
		// assert rather than return an error because this property is statically encoded into the `NodesAtDepth` algorithm
		panic(fmt.Sprintf("match count %d is not even", matches))
	}

	getMatchWinner := func(match LastMatchDTO) int64 {
		switch match.Result {
		case BlackWin:
			return match.BlackID
		case WhiteWin:
			return match.WhiteID
		case Draw:
			return tieBreaker(match)
		default:
			panic(fmt.Sprintf("unknown match result %v", match.Result))
		}
	}

	var nextMatches []AdvanceTournamentMatchDTO
	for i := 0; i+1 < len(matches); i += 2 {
		gameID := MakeGameID()
		matchOne := matches[i]
		matchTwo := matches[i]
		nextMatches = append(nextMatches, AdvanceTournamentMatchDTO{
			GameID:      gameID,
			GameMode:    gameMode,
			PlayerOneID: getMatchWinner(matchOne),
			PlayerTwoID: getMatchWinner(matchTwo),
		})
	}
	return nextMatches
}

type matchmakingInst struct {
	tournamentKey        uuid.UUID
	nextTournamentStatus TournamentStatus
	round                int32
	getInsertionTime     func() time.Time
	matches              []AdvanceTournamentMatchDTO
}

func putTournamentMatches(ctx context.Context, querier sqlc.Querier, inst matchmakingInst) error {
	if len(inst.matches) == 0 {
		return nil
	}

	if err := querier.UpdateTournamentStatus(ctx, sqlc.UpdateTournamentStatusParams{
		TournamentKey: pgtype.UUID{Bytes: inst.tournamentKey, Valid: true},
		Status:        sqlc.TournamentStatusEnum(inst.nextTournamentStatus.String()),
	}); err != nil {
		return fmt.Errorf("update tournament %s status to %s: %w", inst.tournamentKey, inst.nextTournamentStatus, err)
	}

	var matchInsts []sqlc.BatchInsertTournamentMatchParams
	for _, match := range inst.matches {
		matchInsts = append(matchInsts, sqlc.BatchInsertTournamentMatchParams{
			TournamentKey: pgtype.UUID{Bytes: inst.tournamentKey, Valid: true},
			Round:         inst.round,
			GameID:        match.GameID,
			CreatedOn:     pgtype.Timestamptz{Time: inst.getInsertionTime(), Valid: true},
		})
	}

	var batchInsertErrs []error
	querier.BatchInsertTournamentMatch(ctx, matchInsts).Exec(func(i int, err error) {
		if err != nil {
			batchInsertErrs = append(batchInsertErrs, fmt.Errorf("batch %d: inserting tournament matches %+v: %w", i, matchInsts[i], err))
		}
	})
	if err := errors.Join(batchInsertErrs...); err != nil {
		return err
	}

	return pushAdvanceTournamentEvent(ctx, querier, inst.tournamentKey, inst.matches)
}

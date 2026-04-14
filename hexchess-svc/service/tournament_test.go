package svc

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/util/logutil"
	"hexchess-svc/util/testutil"
	"testing"
	"time"
)

func TestCreateTournament(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, ServiceMocks{Entropy: &StableEntropySource{Time: itest.TimeNow}}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	key := uuid.New()

	tournamentID, err := services.CreateTournamentTx(ctx, TournamentInst{
		Key:       key,
		Name:      "Tournament 1",
		Rounds:    2,
		Mode:      ModeCorrespondence1,
		Countdown: time.Hour,
		CreatedOn: itest.TimeNow,
		CreatedBy: 1,
	})
	require.NoError(t, err)

	tournament, err := services.querier.SelectTournamentByID(ctx, pgtype.UUID{Bytes: key, Valid: true})
	require.NoError(t, err)

	wantTournament := sqlc.SelectTournamentByIDRow{
		ID:            tournamentID,
		Name:          "Tournament 1",
		TournamentKey: pgtype.UUID{Bytes: key, Valid: true},
		Rounds:        2,
		Status:        sqlc.TournamentStatusEnum(TournamentLobby.String()),
		Ruleset:       sqlc.TournamentRulesetEnum(TournamentKnockout.String()),
		Countdown:     time.Hour.Milliseconds(),
		CreatedOn:     pgtype.Timestamptz{Time: itest.TimeNow.Local(), Valid: true},
		UpdatedOn:     pgtype.Timestamptz{Time: itest.TimeNow.Local(), Valid: true},
		CreatedBy:     1,
		Mode:          sqlc.ModeEnum(ModeCorrespondence1.String()),
	}
	testutil.Equal(t, wantTournament, tournament)
}

var TournamentLbdChangeSets = []UpdtLbChangeSet{
	{ModeCorrespondence1, 4, 1400},
	{ModeCorrespondence1, 3, 1300},
	{ModeCorrespondence1, 2, 1200},
	{ModeCorrespondence1, 1, 1100},
}

// Tournament2RankedParticipants Ordered by `JoinedOn`, ranked with values in `TournamentLbdChangeSets`
var Tournament2RankedParticipants = []ParticipantDTO{
	{
		UserDTO:    UserDTO{ID: 4, Username: "user4", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        2000,
		HighestElo: 2000,
		Rank:       1,
	},
	{
		UserDTO:    UserDTO{ID: 3, Username: "user3", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        900,
		HighestElo: 900,
		Rank:       2,
	},
	{
		UserDTO:    UserDTO{ID: 2, Username: "user2", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       3,
	},
	{
		UserDTO:    UserDTO{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       4,
	},
}

// Tournament3RankedParticipants Ordered by `JoinedOn`, ranked with values in `TournamentLbdChangeSets`
var Tournament3RankedParticipants = []ParticipantDTO{
	{
		UserDTO:    UserDTO{ID: 2, Username: "user2", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       3,
	},
	{
		UserDTO:    UserDTO{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       4,
	},
}

func TestGetFullTournament(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, ServiceMocks{}, itest.ROPostgres, itest.Redis)
	defer services.Close()

	// seed leaderboard for users fetched in `RetrieveFullTournament` test.
	for _, change := range TournamentLbdChangeSets {
		require.NoError(t, services.SetLeaderboard(context.WithValue(t.Context(), logutil.Trace, t.Name()), change))
	}

	tests := []struct {
		name           string
		tournamentKey  uuid.UUID
		wantTournament FullTournamentDTO
		wantErr        error
	}{
		{
			name:          "EmptyLobbyTournament",
			tournamentKey: itest.Tournament0LobbyKey,
			wantTournament: FullTournamentDTO{
				TournamentDTO: TournamentDTOs[0],
				Participants:  []ParticipantDTO{},
				Matches:       []MatchDTO{},
			},
		},
		{
			name:          "RetrieveFullTournament",
			tournamentKey: itest.Tournament2KnockoutKey,
			wantTournament: FullTournamentDTO{
				TournamentDTO: TournamentDTOs[2],
				Participants:  Tournament2RankedParticipants,
				Matches:       MatchTournament2DTOs, // stable ordering using the `ordering` column
			},
		},
		{
			name:          "RetrieveFullTournamentWithReplay",
			tournamentKey: itest.Tournament5FinishedKey,
			wantTournament: FullTournamentDTO{
				TournamentDTO: TournamentDTOs[5],
				Participants:  Tournament3RankedParticipants,
				Matches:       MatchTournament5DTOs, // stable ordering using the `ordering` column
			},
		},
		{
			name:          "TournamentDoesNotExist",
			tournamentKey: uuid.New(),
			wantErr:       ErrTournamentNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			tournament, err := services.GetFullTournamentByKey(ctx, tt.tournamentKey)

			require.Equal(t, tt.wantErr, err)
			testutil.Equal(t, tt.wantTournament, tournament)
		})
	}
}

func TestGetTournaments(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, ServiceMocks{}, itest.ROPostgres)
	defer services.Close()

	tests := []struct {
		name            string
		participantID   int64
		afterID         int64
		perPage         int32
		wantTournaments []TournamentDTO
		wantErr         error
	}{
		{
			name:          "GetTournaments",
			participantID: -1,
			afterID:       -1,
			perPage:       100,
			wantTournaments: []TournamentDTO{
				// contains all tournament dtos in reverse order
				TournamentDTOs[5],
				TournamentDTOs[4],
				TournamentDTOs[3],
				TournamentDTOs[2],
				TournamentDTOs[1],
				TournamentDTOs[0],
			},
		},
		{
			name:          "GetTournamentsAfterID",
			participantID: -1,
			afterID:       2,
			perPage:       1,
			wantTournaments: []TournamentDTO{
				TournamentDTOs[0],
			},
		},
		{
			name:          "GetTournamentsParticipant",
			participantID: 4,
			afterID:       -1,
			perPage:       100,
			wantTournaments: []TournamentDTO{
				// contains all tournament dtos with user 4 as a participant
				TournamentDTOs[2],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			tournaments, err := services.GetTournaments(ctx, tt.participantID, tt.afterID, tt.perPage)

			require.Equal(t, tt.wantErr, err)
			testutil.Equal(t, tt.wantTournaments, tournaments)
		})
	}
}

func TestBeginTournamentCountdown(t *testing.T) {
	t.Parallel()

	updateTime := itest.TimeNow.Add(time.Minute * 5)

	services, _ := SetupServicesTest(t, ServiceMocks{}, itest.RWPostgres)
	defer services.Close()

	services.entropy = &StableEntropySource{Time: updateTime}

	tests := []struct {
		name                      string
		tournamentKey             uuid.UUID
		userID                    int64
		wantBeginTourneyCountdown BeginTourneyCountdown
		wantErr                   error
		wantTournamentById        sqlc.Tournament
		wantOutboxQueueEvents     []sqlc.OutboxQueue
	}{
		{
			name:          "StatusPreconditionFailed_InProgress",
			tournamentKey: itest.Tournament2KnockoutKey,
			userID:        1,
			wantErr:       ErrInvalidCountdownTournamentStatus,
		},
		{
			name:          "StatusPreconditionFailed_Finished",
			tournamentKey: itest.Tournament5FinishedKey,
			userID:        1,
			wantErr:       ErrInvalidCountdownTournamentStatus,
		},
		{
			name:          "CountdownPermissions",
			tournamentKey: itest.Tournament0LobbyKey,
			userID:        2,
			wantErr:       ErrTournamentCountdownPermissions,
		},
		{
			name:                      "BeginTournamentCountdown",
			tournamentKey:             itest.Tournament0LobbyKey,
			userID:                    1,
			wantBeginTourneyCountdown: BeginTourneyCountdown{TournamentKey: itest.Tournament0LobbyKey},
			wantTournamentById: sqlc.Tournament{
				ID:                 1,
				TournamentKey:      pgtype.UUID{Bytes: itest.Tournament0LobbyKey, Valid: true},
				Name:               "Test Tournament 0",
				Rounds:             2,
				Ruleset:            sqlc.TournamentRulesetEnum(TournamentKnockout.String()),
				Status:             sqlc.TournamentStatusEnum(TournamentScheduled.String()),
				Mode:               sqlc.ModeEnum(ModeCorrespondence1.String()),
				WinnerID:           pgtype.Int8{Valid: false},
				Countdown:          (time.Minute * 5).Milliseconds(),
				CountdownStartedOn: pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
				CreatedOn:          pgtype.Timestamptz{Time: itest.TimeNow.Local(), Valid: true},
				CreatedBy:          1,
				UpdatedOn:          pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
			},
			wantOutboxQueueEvents: []sqlc.OutboxQueue{
				{
					ID:   1,
					Type: sqlc.OutboxQueueTypeEnumTOURNAMENTSCHEDULEDEVENT,
					Data: func() []byte {
						bytes, err := proto.Marshal(&pb.ScheduledTourmmentEvent{
							TournamentKey: itest.Tournament0LobbyKey.String(),
						})
						require.NoError(t, err)
						return bytes
					}(),
					CreatedOn:   pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					ProcessedOn: pgtype.Timestamptz{Valid: false},
					ScheduledOn: pgtype.Timestamptz{Time: updateTime.Add(5 * time.Minute), Valid: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			result, err := services.BeginTournamentCountdownTx(ctx, tt.tournamentKey, tt.userID)

			assert.Equal(t, tt.wantBeginTourneyCountdown, result)
			assert.Equal(t, tt.wantErr, err)

			if tt.wantErr == nil {
				tournament, err := services.querier.SelectTournament(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantTournamentById, tournament)

				outboxEvents, err := services.querier.SelectALLOutboxQueue(ctx)
				require.NoError(t, err)

				testutil.Equal(t, tt.wantOutboxQueueEvents, outboxEvents)
			}
		})
	}
}

func TestJoinTournament(t *testing.T) {
	t.Parallel()

	updateTime := itest.TimeNow.Add(time.Minute * 5)

	services, _ := SetupServicesTest(t, ServiceMocks{}, itest.RWPostgres)
	defer services.Close()

	services.entropy = &StableEntropySource{Time: updateTime}

	tests := []struct {
		name             string
		inst             JoinTournamentInst
		wantResult       JoinTournamentResult
		wantErr          error
		wantParticipants []sqlc.TournamentParticipant
	}{
		{
			name: "StatusPreconditionFailed",
			inst: JoinTournamentInst{
				TournamentKey: itest.Tournament3RoundRobinKey,
				JoiningUserID: 1,
				InsertionTime: updateTime,
			},
			wantErr: ErrTournamentNotLobby,
		},
		{
			name: "ReachedMaxParticipants",
			inst: JoinTournamentInst{
				TournamentKey: itest.Tournament1LobbyFilledKey,
				JoiningUserID: 1,
				InsertionTime: updateTime,
			},
			wantErr: ErrTooManyParticipants,
		},
		{
			name: "JoinedTournament",
			inst: JoinTournamentInst{
				TournamentKey: itest.Tournament0LobbyKey,
				JoiningUserID: 1,
				InsertionTime: updateTime,
			},
			wantResult: JoinTournamentResult{TournamentKey: itest.Tournament0LobbyKey, Mode: ModeCorrespondence1},
			wantParticipants: []sqlc.TournamentParticipant{
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament0LobbyKey, Valid: true},
					UserID:        1,
					JoinedOn:      pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			result, err := services.JoinTournamentTx(ctx, tt.inst)

			assert.Equal(t, tt.wantResult, result)
			assert.Equal(t, tt.wantErr, err)

			if tt.wantErr == nil {
				participants, err := services.querier.SelectParticipants(ctx, pgtype.UUID{Bytes: tt.inst.TournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantParticipants, participants)
			}
		})
	}
}

func TestStartTournament(t *testing.T) {

}

func TestAdvanceTournament(t *testing.T) {

}

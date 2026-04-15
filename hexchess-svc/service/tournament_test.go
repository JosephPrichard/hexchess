package svc

import (
	"context"
	"github.com/google/go-cmp/cmp/cmpopts"
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

	services, _ := SetupServicesTest(t, ServiceMocks{Entropy: &StableEntropySource{CurrTime: itest.TimeNow}}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	key := uuid.New()

	tournamentID, err := services.CreateTournamentTx(ctx, TournamentInst{
		Key:       key,
		Name:      "Tournaments 1",
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
		Name:          "Tournaments 1",
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
		wantTournament FullTournament
		wantErr        error
	}{
		{
			name:          "EmptyLobbyTournament",
			tournamentKey: itest.Tournament0LobbyKey,
			wantTournament: FullTournament{
				Tournament:   Tournaments[0],
				Participants: []Participant{},
				Matches:      []Match{},
			},
		},
		{
			name:          "RetrieveFullTournament",
			tournamentKey: itest.Tournament5InProgressKnockoutKey,
			wantTournament: FullTournament{
				Tournament:   Tournaments[5],
				Participants: Tournament5RankedParticipants,
				Matches:      MatchTournament5, // stable ordering using the `ordering` column
			},
		},
		{
			name:          "RetrieveFullTournamentWithReplay",
			tournamentKey: itest.Tournament8FinishedKey,
			wantTournament: FullTournament{
				Tournament:   Tournaments[8],
				Participants: Tournament8RankedParticipants,
				Matches:      MatchTournament8, // stable ordering using the `ordering` column
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
		wantTournaments []Tournament
		wantErr         error
	}{
		{
			name:          "GetTournaments",
			participantID: -1,
			afterID:       -1,
			perPage:       100,
			wantTournaments: []Tournament{
				// contains all tournament  in reverse order
				Tournaments[8],
				Tournaments[7],
				Tournaments[6],
				Tournaments[5],
				Tournaments[4],
				Tournaments[3],
				Tournaments[2],
				Tournaments[1],
				Tournaments[0],
			},
		},
		{
			name:          "GetTournamentsAfterID",
			participantID: -1,
			afterID:       2,
			perPage:       1,
			wantTournaments: []Tournament{
				Tournaments[0],
			},
		},
		{
			name:          "GetTournamentsParticipant",
			participantID: 4,
			afterID:       -1,
			perPage:       100,
			wantTournaments: []Tournament{
				// contains all tournament with user 4 as a participant
				Tournaments[5],
				Tournaments[2],
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

var cmpOptsOutboxEvent = cmpopts.IgnoreFields(sqlc.OutboxQueue{}, "ID")

func TestBeginTournamentCountdown(t *testing.T) {
	t.Parallel()

	updateTime := itest.TimeNow.Add(time.Minute * 5)

	services, _ := SetupServicesTest(t, ServiceMocks{}, itest.RWPostgres)
	defer services.Close()

	services.entropy = &StableEntropySource{CurrTime: updateTime}

	tests := []struct {
		name                      string
		tournamentKey             uuid.UUID
		userID                    int64
		wantBeginTourneyCountdown BeginTourneyCountdown
		wantErr                   error
		wantTournamenStatus       sqlc.TournamentStatusEnum
		wantOutboxQueueEvents     []sqlc.OutboxQueue
	}{
		{
			name:          "StatusPreconditionFailed_Scheduled",
			tournamentKey: itest.Tournament2ScheduledKnockoutKey,
			userID:        1,
			wantErr:       ErrInvalidCountdownTournamentStatus,
		},
		{
			name:          "StatusPreconditionFailed_InProgress",
			tournamentKey: itest.Tournament5InProgressKnockoutKey,
			userID:        1,
			wantErr:       ErrInvalidCountdownTournamentStatus,
		},
		{
			name:          "StatusPreconditionFailed_Finished",
			tournamentKey: itest.Tournament8FinishedKey,
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
			wantTournamenStatus:       sqlc.TournamentStatusEnumSCHEDULED,
			wantOutboxQueueEvents: []sqlc.OutboxQueue{
				{
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
					ScheduledOn: pgtype.Timestamptz{Time: updateTime.Add(time.Minute*5 + time.Second), Valid: true},
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
				status, err := services.querier.SelectTournamentStatus(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantTournamenStatus, status)

				outboxEvents, err := services.querier.SelectALLOutboxQueue(ctx)
				require.NoError(t, err)

				testutil.Equal(t, tt.wantOutboxQueueEvents, outboxEvents, cmpOptsOutboxEvent)
			}
		})
	}
}

func TestJoinTournament(t *testing.T) {
	t.Parallel()

	updateTime := itest.TimeNow.Add(time.Minute * 5)

	services, _ := SetupServicesTest(t, ServiceMocks{}, itest.RWPostgres)
	defer services.Close()

	services.entropy = &StableEntropySource{CurrTime: updateTime}

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
				TournamentKey: itest.Tournament3ScheduledRoundRobinKey,
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

var sqlcTournamentMatchCmpOpts = cmpopts.IgnoreFields(sqlc.TournamentMatch{}, "Ordering")

func TestStartTournament(t *testing.T) {
	t.Parallel()

	updateTime := itest.TimeNow.Add(time.Minute * 5)

	setupServices := func() (*HexchessServices, func()) {
		services, _ := SetupServicesTest(t, ServiceMocks{}, itest.RWPostgres)

		services.entropy = &StableEntropySource{CurrTime: updateTime}
		return services, services.Close
	}

	tests := []struct {
		name                  string
		tournamentKey         uuid.UUID
		wantErr               error
		wantStatus            sqlc.TournamentStatusEnum
		wantMatches           []sqlc.TournamentMatch
		wantOutboxQueueEvents []sqlc.OutboxQueue
	}{
		{
			name:          "StatusPreconditionFailed_Lobby",
			tournamentKey: itest.Tournament1LobbyFilledKey,
			wantErr:       MatchInvariantError{TournamentKey: itest.Tournament1LobbyFilledKey, Err: ErrInvalidStartTournamentStatus},
		},
		{
			name:          "StatusPreconditionFailed_InProgress",
			tournamentKey: itest.Tournament5InProgressKnockoutKey,
			wantErr:       MatchInvariantError{TournamentKey: itest.Tournament5InProgressKnockoutKey, Err: ErrInvalidStartTournamentStatus},
		},
		{
			name:          "StatusPreconditionFailed_Finished",
			tournamentKey: itest.Tournament8FinishedKey,
			wantErr:       MatchInvariantError{TournamentKey: itest.Tournament8FinishedKey, Err: ErrInvalidStartTournamentStatus},
		},
		{
			name:          "StartingKnockoutTournament",
			tournamentKey: itest.Tournament2ScheduledKnockoutKey,
			wantStatus:    sqlc.TournamentStatusEnumINPROGRESS,
			wantMatches: []sqlc.TournamentMatch{
				// tournament has 2 rounds with join order of [1,2,3,4], so starting the tournament creates 2 rounds wso the matches go 1-2, 3-4
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
					GameID:        "mock-1",
					Round:         1,
					CreatedOn:     pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					WhiteID:       3,
					BlackID:       4,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
					GameID:        "mock-2",
					Round:         1,
					CreatedOn:     pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					WhiteID:       5,
					BlackID:       6,
				},
			},
			wantOutboxQueueEvents: []sqlc.OutboxQueue{
				{
					Type: sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data: func() []byte {
						bytes, err := proto.Marshal(&pb.CreateTournamentMatchesEvent{
							TournamentKey: itest.Tournament2ScheduledKnockoutKey.String(),
							Matches: []*pb.CreateTournamentMatch{
								{
									GameId:   "mock-1",
									WhiteId:  3,
									BlackId:  4,
									GameMode: ModeCorrespondence1.String(),
								},
								{
									GameId:   "mock-2",
									WhiteId:  5,
									BlackId:  6,
									GameMode: ModeCorrespondence1.String(),
								},
							},
						})
						require.NoError(t, err)
						return bytes
					}(),
					CreatedOn:   pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					ProcessedOn: pgtype.Timestamptz{Valid: false},
					ScheduledOn: pgtype.Timestamptz{Valid: false},
				},
			},
		},
		{
			name:          "StartingRoundRobinTournament",
			tournamentKey: itest.Tournament3ScheduledRoundRobinKey,
			wantStatus:    sqlc.TournamentStatusEnumINPROGRESS,
			wantMatches: []sqlc.TournamentMatch{
				// tournament has 4 participants with join order of [1,2,3,4], so the matches go 1-2, 3-4
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament3ScheduledRoundRobinKey, Valid: true},
					GameID:        "mock-1",
					Round:         1,
					CreatedOn:     pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					WhiteID:       1,
					BlackID:       2,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament3ScheduledRoundRobinKey, Valid: true},
					GameID:        "mock-2",
					Round:         1,
					CreatedOn:     pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					WhiteID:       5,
					BlackID:       6,
				},
			},
			wantOutboxQueueEvents: []sqlc.OutboxQueue{
				{
					Type: sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data: func() []byte {
						bytes, err := proto.Marshal(&pb.CreateTournamentMatchesEvent{
							TournamentKey: itest.Tournament3ScheduledRoundRobinKey.String(),
							Matches: []*pb.CreateTournamentMatch{
								{
									GameId:   "mock-1",
									WhiteId:  1,
									BlackId:  2,
									GameMode: ModeCorrespondence1.String(),
								},
								{
									GameId:   "mock-2",
									WhiteId:  5,
									BlackId:  6,
									GameMode: ModeCorrespondence1.String(),
								},
							},
						})
						require.NoError(t, err)
						return bytes
					}(),
					CreatedOn:   pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					ProcessedOn: pgtype.Timestamptz{Valid: false},
					ScheduledOn: pgtype.Timestamptz{Valid: false},
				},
			},
		},
		{
			name:          "StartingSwissTournament",
			tournamentKey: itest.Tournament4ScheduledSwissKey,
			wantStatus:    sqlc.TournamentStatusEnumINPROGRESS,
			wantMatches: []sqlc.TournamentMatch{
				// tournament has 4 participants with elo ordering of [6,5,2,1] for Correspondence1, so the matches go 6-2, 5-1
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament4ScheduledSwissKey, Valid: true},
					GameID:        "mock-1",
					Round:         1,
					CreatedOn:     pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					WhiteID:       6,
					BlackID:       2,
				},
				{
					TournamentKey: pgtype.UUID{Bytes: itest.Tournament4ScheduledSwissKey, Valid: true},
					GameID:        "mock-2",
					Round:         1,
					CreatedOn:     pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					WhiteID:       5,
					BlackID:       1,
				},
			},
			wantOutboxQueueEvents: []sqlc.OutboxQueue{
				{
					Type: sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data: func() []byte {
						bytes, err := proto.Marshal(&pb.CreateTournamentMatchesEvent{
							TournamentKey: itest.Tournament4ScheduledSwissKey.String(),
							Matches: []*pb.CreateTournamentMatch{
								{
									GameId:   "mock-1",
									WhiteId:  6,
									BlackId:  2,
									GameMode: ModeCorrespondence1.String(),
								},
								{
									GameId:   "mock-2",
									WhiteId:  5,
									BlackId:  1,
									GameMode: ModeCorrespondence1.String(),
								},
							},
						})
						require.NoError(t, err)
						return bytes
					}(),
					CreatedOn:   pgtype.Timestamptz{Time: updateTime.Local(), Valid: true},
					ProcessedOn: pgtype.Timestamptz{Valid: false},
					ScheduledOn: pgtype.Timestamptz{Valid: false},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// service is constructed once per test since tests share the outbox queue and we need an assertion per outbox queue.
			services, closer := setupServices()
			defer closer()

			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			err := services.StartTournamentTx(ctx, tt.tournamentKey)

			assert.Equal(t, tt.wantErr, err)

			if tt.wantErr == nil {
				matches, err := services.querier.SelectMatches(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantMatches, matches, sqlcTournamentMatchCmpOpts)

				status, err := services.querier.SelectTournamentStatus(ctx, pgtype.UUID{Bytes: tt.tournamentKey, Valid: true})
				require.NoError(t, err)

				testutil.Equal(t, tt.wantStatus, status)

				outboxEvents, err := services.querier.SelectALLOutboxQueue(ctx)
				require.NoError(t, err)

				testutil.Equal(t, tt.wantOutboxQueueEvents, outboxEvents, cmpOptsOutboxEvent)
			}
		})
	}
}

func TestAdvanceTournament(t *testing.T) {

}

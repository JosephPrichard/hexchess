package svc

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/internal/logutil"
	"hexchess-svc/internal/testutil"
	"hexchess-svc/itest"
	"testing"
	"time"
)

func TestCreateTournament(t *testing.T) {
	t.Parallel()

	services := SetupServicesTest(t, itest.RWPostgres)
	defer services.Close()

	services.EntropySource = &StableEntropySource{Time: itest.TimeNow}

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	tournamentID, err := services.CreateTournament(ctx, TournamentInst{
		TournamentKey: "tournament-1",
		Name:          "Tournament 1",
		Depth:         2,
		Mode:          ModeCorrespondence1,
		ScheduledIn:   time.Hour,
		CreatedOn:     itest.TimeNow,
		CreatedBy:     1,
	})
	require.NoError(t, err)

	tournament, err := services.Querier.SelectTournamentById(ctx, tournamentID)
	require.NoError(t, err)

	wantTournament := sqlc.SelectTournamentByIdRow{
		ID:            tournamentID,
		Name:          "Tournament 1",
		TournamentKey: "tournament-1",
		Depth:         2,
		Status:        sqlc.TournamentStatusEnum(TournamentLobby.String()),
		ScheduledOn:   pgtype.Timestamptz{Time: itest.TimeNow.Add(time.Hour).Local(), Valid: true},
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

var RankedParticipants = []ParticipantDTO{
	{
		TournamentID: 2,
		JoinedOn:     itest.TimeNow,
		LbdUserDTO: LbdUserDTO{
			UserDTO:    UserDTO{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
			Elo:        1000,
			HighestElo: 1000,
			Rank:       4,
		},
	},
	{
		TournamentID: 2,
		JoinedOn:     itest.TimeNow,
		LbdUserDTO: LbdUserDTO{
			UserDTO:    UserDTO{ID: 2, Username: "user2", Country: "us", JoinedOn: itest.TimeNow},
			Elo:        1000,
			HighestElo: 1000,
			Rank:       3,
		},
	},
	{
		TournamentID: 2,
		JoinedOn:     itest.TimeNow,
		LbdUserDTO: LbdUserDTO{
			UserDTO:    UserDTO{ID: 3, Username: "user3", Country: "us", JoinedOn: itest.TimeNow},
			Elo:        900,
			HighestElo: 900,
			Rank:       2,
		},
	},
	{
		TournamentID: 2,
		JoinedOn:     itest.TimeNow,
		LbdUserDTO: LbdUserDTO{
			UserDTO:    UserDTO{ID: 4, Username: "user4", Country: "us", JoinedOn: itest.TimeNow},
			Elo:        2000,
			HighestElo: 2000,
			Rank:       1,
		},
	},
}

func TestGetFullTournament(t *testing.T) {
	t.Parallel()

	services := SetupServicesTest(t, itest.ROPostgres, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// seed leaderboard for users fetched in `RetrieveFullTournament` test.
	for _, change := range TournamentLbdChangeSets {
		require.NoError(t, services.SetLeaderboard(ctx, change))
	}

	tests := []struct {
		name           string
		tournamentID   int64
		wantTournament FullTournamentDTO
		wantErr        error
	}{
		{
			name:         "EmptyLobbyTournament",
			tournamentID: 1,
			wantTournament: FullTournamentDTO{
				TournamentDTO: TournamentDTOs[0],
				Participants:  []ParticipantDTO{},
				Matches:       []MatchDTO{},
			},
		},
		{
			name:         "RetrieveFullTournament",
			tournamentID: 2,
			wantTournament: FullTournamentDTO{
				TournamentDTO: TournamentDTOs[1],
				Participants:  RankedParticipants,
				Matches:       []MatchDTO{MatchDtos[0], MatchDtos[1]},
			},
		},
		{
			name:         "TournamentDoesNotExist",
			tournamentID: -999,
			wantErr:      ErrNoTournament,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tournament, err := services.GetTournamentByIDWithRanks(ctx, tt.tournamentID)

			assert.Equal(t, tt.wantErr, err)
			testutil.Equal(t, tt.wantTournament, tournament)
		})
	}
}

func TestGetTournaments(t *testing.T) {
	t.Parallel()

	services := SetupServicesTest(t, itest.ROPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	tests := []struct {
		name            string
		participantID   int64
		afterID         int64
		wantTournaments []TournamentDTO
		wantErr         error
	}{
		{
			name:          "GetTournaments",
			participantID: -1,
			afterID:       -1,
			wantTournaments: []TournamentDTO{
				TournamentDTOs[2],
			},
		},
		{
			name:          "GetTournamentsAfterID",
			participantID: -1,
			afterID:       2,
			wantTournaments: []TournamentDTO{
				TournamentDTOs[0],
			},
		},
		{
			name:          "GetTournamentsParticipant",
			participantID: 4,
			afterID:       -1,
			wantTournaments: []TournamentDTO{
				TournamentDTOs[1],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tournaments, err := services.GetTournaments(ctx, tt.participantID, tt.afterID, 1)

			assert.Equal(t, tt.wantErr, err)
			testutil.Equal(t, tt.wantTournaments, tournaments)
		})
	}
}

package queue

import (
	"context"
	"errors"
	"hexchess-svc/internal/logutil"
	"hexchess-svc/internal/testutil"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"hexchess-svc/pubsub"
	svc "hexchess-svc/service"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
)

func TestHandleCreateTournamentMatchesEvent(t *testing.T) {
	gameIDOne := "abc-game-one" + uuid.NewString()
	gameIDTwo := "xyz-game-two" + uuid.NewString()

	wantChessOne := model.ChessState{
		ChessMeta: model.ChessMeta{
			ID:          gameIDOne,
			FirstColor:  model.White,
			WhitePlayer: model.PlayerState{ID: 1, Name: "user1", Country: "us", Present: true},
			BlackPlayer: model.PlayerState{ID: 2, Name: "user2", Country: "us", Present: true},
			Mode:        model.ModeCorrespondence1,
		},
	}
	wantChessTwo := model.ChessState{
		ChessMeta: model.ChessMeta{
			ID:          gameIDTwo,
			FirstColor:  model.White,
			WhitePlayer: model.PlayerState{ID: 3, Name: "user3", Country: "us", Present: true},
			BlackPlayer: model.PlayerState{ID: 4, Name: "user4", Country: "us", Present: true},
			Mode:        model.ModeCorrespondence7,
		},
	}

	makeCmpChessStates := func(want []model.ChessState) func(chessState []model.ChessState) bool {
		return func(chessStates []model.ChessState) bool {
			return testutil.Equal(t,
				want,
				chessStates,
				cmpopts.IgnoreFields(model.ChessState{}, "InitialBoard", "Game"),
				model.ChessMetaCmpOpt,
			)
		}
	}

	tests := []struct {
		name       string
		input      *pb.CreateTournamentMatchesEvent
		setupMocks func(*gomock.Controller) svc.HexchessAPI
		wantErr    bool
	}{
		{
			name: "CreatesTournamentMatches",
			input: &pb.CreateTournamentMatchesEvent{
				TournamentKey: uuid.NewString(),
				Matches: []*pb.CreateTournamentMatch{
					{
						GameId:   gameIDOne,
						WhiteId:  1,
						BlackId:  2,
						GameMode: model.ModeCorrespondence1.String(),
					},
					{
						GameId:   gameIDTwo,
						WhiteId:  3,
						BlackId:  4,
						GameMode: model.ModeCorrespondence7.String(),
					},
				},
			},
			setupMocks: func(ctrl *gomock.Controller) svc.HexchessAPI {
				hexchessAPI := svc.NewMockHexchessAPI(ctrl)

				cmpChessStates := makeCmpChessStates([]model.ChessState{wantChessOne, wantChessTwo})

				hexchessAPI.EXPECT().
					SelectUsersByIDs(gomock.Any(), gomock.Eq([]int64{1, 2, 3, 4})).
					Return([]model.User{
						{ID: 1, Username: "user1", Country: "us"},
						{ID: 2, Username: "user2", Country: "us"},
						{ID: 3, Username: "user3", Country: "us"},
						{ID: 4, Username: "user4", Country: "us"},
					}, nil)
				hexchessAPI.EXPECT().
					SetManyChessStates(gomock.Any(), gomock.Cond(cmpChessStates)).
					Return(nil)

				return hexchessAPI
			},
		},
		{
			name: "InvalidUserFailsToCreateTournamentMatches",
			input: &pb.CreateTournamentMatchesEvent{
				TournamentKey: uuid.NewString(),
				Matches: []*pb.CreateTournamentMatch{
					{
						GameId:   gameIDOne,
						WhiteId:  1,
						BlackId:  9000, // invalid user id
						GameMode: model.ModeCorrespondence1.String(),
					},
				},
			},
			setupMocks: func(ctrl *gomock.Controller) svc.HexchessAPI {
				hexchessAPI := svc.NewMockHexchessAPI(ctrl)

				hexchessAPI.EXPECT().
					SelectUsersByIDs(gomock.Any(), gomock.Eq([]int64{1, 9000})).
					Return([]model.User{
						{ID: 1, Username: "user1", Country: "us"},
					}, nil)

				return hexchessAPI
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := t.Context()

			bytes, err := proto.Marshal(tt.input)
			require.NoError(t, err)

			h := EventHandler{Services: tt.setupMocks(ctrl)}

			err = h.HandleCreateTournamentMatchesEvent(ctx, bytes)

			if tt.wantErr != (err != nil) {
				t.Fatalf("expected error: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestHandleHandleAdvanceTournamentEvent(t *testing.T) {
	tournamentOneKey := uuid.New()
	tournamentTwoKey := uuid.New()

	tests := []struct {
		name          string
		event         *pb.ScheduledTourmmentEvent
		setupMocks    func(*gomock.Controller) (svc.HexchessAPI, pubsub.BroadcasterAPI)
		wantBroadcast *pb.TournamentOutput
		wantErr       error
	}{
		{
			name:  "AdvanceTournament",
			event: &pb.ScheduledTourmmentEvent{TournamentKey: tournamentOneKey.String()},
			setupMocks: func(ctrl *gomock.Controller) (svc.HexchessAPI, pubsub.BroadcasterAPI) {
				mockHexchessAPI := svc.NewMockHexchessAPI(ctrl)
				mockHexchessAPI.EXPECT().
					AdvanceTournament(gomock.Any(), gomock.Eq(tournamentOneKey)).
					Return(nil)

				mockBroadcasterAPI := pubsub.NewMockBroadcasterAPI(ctrl)
				mockBroadcasterAPI.EXPECT().
					BroadcastTournament(gomock.Any(), gomock.Eq(&pb.TournamentOutput{
						TournamentKey: tournamentOneKey.String(),
						Value:         &pb.TournamentOutput_Start{Start: &pb.StartTourneyOutput{}},
					})).
					Return(nil)

				return mockHexchessAPI, mockBroadcasterAPI
			},
		},
		{
			name:  "AdvanceTournamentNonRetryableError",
			event: &pb.ScheduledTourmmentEvent{TournamentKey: tournamentTwoKey.String()},
			setupMocks: func(ctrl *gomock.Controller) (svc.HexchessAPI, pubsub.BroadcasterAPI) {
				mockHexchessAPI := svc.NewMockHexchessAPI(ctrl)
				mockHexchessAPI.EXPECT().
					AdvanceTournament(gomock.Any(), gomock.Eq(tournamentTwoKey)).
					Return(svc.MatchInvariantError{Err: errors.New("test error")})

				mockBroadcasterAPI := pubsub.NewMockBroadcasterAPI(ctrl)
				mockBroadcasterAPI.EXPECT().
					BroadcastTournament(gomock.Any(), gomock.Eq(&pb.TournamentOutput{
						TournamentKey: tournamentTwoKey.String(),
						Value: &pb.TournamentOutput_Error{
							Error: &pb.ErrorOutput{Message: model.ErrAdvanceTournamentCode.Error()},
						},
					})).
					Return(nil)

				return mockHexchessAPI, mockBroadcasterAPI
			},
			wantErr: NonRetryableQueueError{
				Err: svc.MatchInvariantError{Err: errors.New("test error")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := t.Context()

			bytes, err := proto.Marshal(tt.event)
			require.NoError(t, err)

			services, broadcaster := tt.setupMocks(ctrl)
			h := EventHandler{Services: services, Broadcaster: broadcaster}

			err = h.HandleAdvanceTournamentEvent(ctx, bytes)

			require.Equal(t, tt.wantErr, err)
		})
	}
}

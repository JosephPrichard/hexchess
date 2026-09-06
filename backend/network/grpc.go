package network

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/rpc"
	"hexchess-svc/service/matchmaking"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/slogutil"
	"log/slog"

	"github.com/google/uuid"
)

type GRPCServer struct {
	rpc.UnimplementedMatchmakerServiceServer
	matchmakerService *matchmaking.MatchmakerService
}

func NewGRPCServer() GRPCServer {
	return GRPCServer{matchmakerService: matchmaking.NewMatchmakerService()}
}

func (server *GRPCServer) MatchmakingStream(stream rpc.MatchmakerService_MatchmakingStreamServer) error {
	ctx := context.WithValue(context.Background(), slogutil.Trace, uuid.NewString())

	state := &matchmakingStreamState{
		service:      server.matchmakerService,
		responseChan: make(chan matchmaking.MatchResponse, 1),
	}

	go func() {
		for matchResponse := range state.responseChan {
			response := mapMatchmakingResponse(matchResponse)

			err := stream.Send(response)
			if err != nil {
				slog.ErrorContext(ctx, "failed to send response in matchmaking stream", "error", err)
			}
		}
	}()

	for {
		request, err := stream.Recv()
		if err != nil {
			slog.WarnContext(ctx, "failed to recv request in matchmaking stream", "error", err)
			return err
		}
		handleMatchmakingRequest(ctx, state, request)
	}
}

type matchmakingStreamState struct {
	service *matchmaking.MatchmakerService

	responseChan chan matchmaking.MatchResponse

	userID int64
	mode   model.GameMode
}

func handleMatchmakingRequest(ctx context.Context, state *matchmakingStreamState, request *rpc.MatchmakingRequest) {
	switch value := request.Value.(type) {
	case *rpc.MatchmakingRequest_Begin:
		input := value.Begin

		inputMode, err := enum.Parse(input.Mode, model.GameModeEnums)
		if err != nil {
			slog.ErrorContext(ctx, "invalid mode in recv begin request - dropping message", "error", err)
			return
		}

		state.userID = input.UserId
		state.mode = inputMode

		state.service.SendMatchRequest(matchmaking.MatchmakingRequest{
			Kind:         matchmaking.MatchRequestBegin,
			Mode:         state.mode,
			UserID:       state.userID,
			UserElo:      input.UserElo,
			ResponseChan: state.responseChan,
		})
	case *rpc.MatchmakingRequest_Confirm:
		state.service.SendMatchRequest(matchmaking.MatchmakingRequest{
			Kind:   matchmaking.MatchRequestConfirmation,
			Mode:   state.mode,
			UserID: state.userID,
		})
	}
}

func mapMatchmakingResponse(matchResponse matchmaking.MatchResponse) *rpc.MatchmakingResponse {
	response := &rpc.MatchmakingResponse{}

	switch matchResponse.Kind {
	case matchmaking.MatchResponseProposal:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Begin{
			Begin: &rpc.MatchmakingProposalResponse{
				UserOneId: matchResponse.UserOneID,
				UserTwoId: matchResponse.UserTwoID,
			},
		}}
	case matchmaking.MatchResponseConfirmation:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Confirm{Confirm: &rpc.MatchmakingConfirmResponse{}}}
	}

	return response
}

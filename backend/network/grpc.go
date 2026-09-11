package network

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/rpc"
	"hexchess-svc/service"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/slogutil"
	"log/slog"

	"github.com/google/uuid"
)

type GRPCServer struct {
	rpc.UnimplementedMatchmakerServiceServer
	matchmakerService *service.MatchmakerService
}

func NewGRPCServer() GRPCServer {
	return GRPCServer{matchmakerService: service.NewMatchmakerService()}
}

func (server *GRPCServer) MatchmakingStream(stream rpc.MatchmakerService_MatchmakingStreamServer) error {
	ctx := context.WithValue(context.Background(), slogutil.Trace, uuid.NewString())

	state := &MatchmakingStreamState{
		service:      server.matchmakerService,
		responseChan: make(chan service.MatchResponse, 1),
	}

	defer func() {
		slog.InfoContext(ctx, "matchmaking stream :: engaging cancellation")
		state.service.SendMatchRequest(service.NewMatchRequestCancel(state.matchmakeID))
	}()

	go func() {
		for matchResponse := range state.responseChan {
			response := mapMatchmakingResponse(matchResponse)
			slog.InfoContext(ctx, "matchmaking stream :: sending response", "response", response)

			if err := stream.Send(response); err != nil {
				slog.ErrorContext(ctx, "matchmaking stream :: failed to send response", "error", err)
			}
		}
	}()

	for {
		request, err := stream.Recv()
		if err != nil {
			slog.WarnContext(ctx, "matchmaking stream :: failed to receive request", "error", err)
			return err
		}
		slog.InfoContext(ctx, "matchmaking stream :: handling request", "request", request)

		handleMatchmakingRequest(ctx, state, request)
	}
}

type MatchmakingStreamState struct {
	service      *service.MatchmakerService
	responseChan chan service.MatchResponse

	matchmakeID service.MatchmakeID
}

func handleMatchmakingRequest(ctx context.Context, state *MatchmakingStreamState, request *rpc.MatchmakingRequest) {
	switch value := request.Value.(type) {
	case *rpc.MatchmakingRequest_Begin:
		inputMode, err := enum.Parse(value.Begin.Mode, model.GameModeEnums)
		if err != nil {
			slog.ErrorContext(ctx, "matchmaking stream :: invalid mode in recv begin request - dropping message", "error", err)
			return
		}

		state.matchmakeID = service.MatchmakeID{UserID: value.Begin.UserId, Mode: inputMode}

		state.service.SendMatchRequest(service.NewMatchRequestBegin(state.matchmakeID, value.Begin.UserElo, state.responseChan))
	case *rpc.MatchmakingRequest_Confirm:
		state.service.SendMatchRequest(service.NewMatchRequestConfirmation(state.matchmakeID))
	}
}

func mapMatchmakingResponse(matchResponse service.MatchResponse) *rpc.MatchmakingResponse {
	response := &rpc.MatchmakingResponse{}

	switch matchResponse.Kind {
	case service.MatchResponseProposal:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Begin{
			Begin: &rpc.MatchmakingProposalResponse{
				UserOneId: matchResponse.UserOneID,
				UserTwoId: matchResponse.UserTwoID,
			},
		}}
	case service.MatchResponseConfirmation:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Confirm{Confirm: &rpc.MatchmakingConfirmResponse{}}}
	}

	return response
}

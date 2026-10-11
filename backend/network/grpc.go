package network

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/rpc"
	"hexchess-svc/service"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/slogutil"
	"io"
	"log/slog"

	"github.com/google/uuid"
)

type GRPCServer struct {
	rpc.UnimplementedMatchmakerServiceServer
	matchmakerService *service.MatchmakerService
}

func NewGRPCServer(matchmakerService *service.MatchmakerService) GRPCServer {
	return GRPCServer{matchmakerService: matchmakerService}
}

func (server *GRPCServer) MatchmakingStream(stream rpc.MatchmakerService_MatchmakingStreamServer) error {
	ctx, cancel := context.WithCancel(slogutil.TracedContext(context.Background(), uuid.NewString()))
	defer cancel()

	state := &MatchmakingStreamState{
		service:      server.matchmakerService,
		responseChan: make(chan service.MatchResponse, 1),
	}

	// writer: send matchmaking service responses through stream
	go func() {
		for matchResponse := range state.responseChan {
			response := mapMatchmakingResponse(matchResponse)
			slog.InfoContext(ctx, "sending matchmaking stream response", "state", state, "response", response)

			if err := stream.Send(response); err != nil {
				slog.ErrorContext(ctx, "failed to send matchmaking stream response", "state", state, "error", err)
			}
		}
		cancel() // signal reader to stop listening to requests
	}()

	shutdown := func() error {
		slog.InfoContext(ctx, "shutting down matchmaking stream", "state", state)
		state.service.SendMatchRequest(service.NewMatchRequestCancel(state.matchmakeID.UserID, state.matchmakeID.Mode))
		return nil
	}

	// reader: send each matchmaking request to service and send the final request on shutdown
	for {
		select {
		case <-ctx.Done():
			return shutdown()
		default:
			request, err := stream.Recv()
			if err == io.EOF {
				return shutdown()
			} else if err != nil {
				slog.WarnContext(ctx, "failed to receive matchmaking stream request", "state", state, "error", err)
				return err
			}

			slog.InfoContext(ctx, "handling matchmaking stream request", "state", state, "request", request)
			handleMatchmakingRequest(ctx, state, request)
		}
	}
}

type MatchmakingStreamState struct {
	service      *service.MatchmakerService
	responseChan chan service.MatchResponse

	matchmakeID MatchmakingID
}

type MatchmakingID struct {
	UserID int64          `json:"userID"`
	Mode   model.GameMode `json:"mode"`
}

func handleMatchmakingRequest(ctx context.Context, state *MatchmakingStreamState, request *rpc.MatchmakingRequest) {
	switch value := request.Value.(type) {
	case *rpc.MatchmakingRequest_Begin:
		inputMode, err := enum.Parse(value.Begin.Mode, model.GameModeEnums)
		if err != nil {
			slog.ErrorContext(ctx, "invalid mode in recv begin request - dropping message", "error", err)
			return
		}

		state.matchmakeID = MatchmakingID{UserID: value.Begin.UserId, Mode: inputMode}

		state.service.SendMatchRequest(service.NewMatchRequestBegin(state.matchmakeID.UserID, state.matchmakeID.Mode, value.Begin.UserElo, state.responseChan))
	case *rpc.MatchmakingRequest_Confirm:
		if value.Confirm.Accept {
			state.service.SendMatchRequest(service.NewMatchRequestConfirmation(state.matchmakeID.UserID, state.matchmakeID.Mode))
		}
	}
}

func mapMatchmakingResponse(matchResponse service.MatchResponse) *rpc.MatchmakingResponse {
	response := &rpc.MatchmakingResponse{}

	switch matchResponse.Kind {
	case service.MatchResponseJoin:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Join{
			Join: &rpc.MatchmakingJoinResponse{},
		}}
	case service.MatchResponseProposal:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Proposal{
			Proposal: &rpc.MatchmakingProposalResponse{
				UserOneId: matchResponse.UserOneID,
				UserTwoId: matchResponse.UserTwoID,
			},
		}}
	case service.MatchResponseConfirmation:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Confirm{
			Confirm: &rpc.MatchmakingConfirmResponse{},
		}}
	case service.MatchResponseCancel:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Cancel{
			Cancel: &rpc.MatchmakingCancelResponse{},
		}}
	}

	return response
}

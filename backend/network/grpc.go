package network

import (
	"context"
	"hexchess-svc/gamedata"
	"hexchess-svc/matchmaker"
	"hexchess-svc/rpc"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/slogutil"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"go.uber.org/atomic"
)

type GRPCServer struct {
	rpc.UnimplementedMatchmakerServiceServer
	matchmakerService *matchmaker.MatchmakerService
}

func NewGRPCServer(matchmakerService *matchmaker.MatchmakerService) GRPCServer {
	return GRPCServer{matchmakerService: matchmakerService}
}

func (server *GRPCServer) MatchmakingStream(stream rpc.MatchmakerService_MatchmakingStreamServer) error {
	ctx, cancel := context.WithCancel(slogutil.TracedContext(context.Background(), uuid.NewString()))
	defer cancel()

	responseChan := make(chan matchmaker.MatchResponse, 1)
	state := newMatchmakingStreamState(server.matchmakerService, responseChan)

	cleanup := func() {
		// both close operations are idempotent
		matchmakingID := state.getMatchmakingID()
		cancelRequest := matchmaker.MatchmakingRequest{Kind: matchmaker.CancelMatchRequest, UserID: matchmakingID.UserID, Mode: matchmakingID.Mode}
		server.matchmakerService.SendMatchRequest(cancelRequest) // send close signal to reader
		cancel()                                                 // send close signal to writer
	}

	// writer: send matchmaking service responses through stream
	go func() {
		defer cleanup()

		for matchResponse := range responseChan {
			response := mapMatchmakingResponse(matchResponse)
			slog.InfoContext(ctx, "sending matchmaking stream response", "response", response)

			if err := stream.Send(response); err != nil {
				slog.ErrorContext(ctx, "failed to send matchmaking stream response", "error", err)
			}
		}
	}()

	// reader: send each matchmaking request to service and send the final request on shutdown
	defer cleanup()
ReadLoop:
	for {
		select {
		case <-ctx.Done():
			break ReadLoop
		default:
			request, err := stream.Recv()
			if err == io.EOF {
				break ReadLoop
			} else if err != nil {
				slog.WarnContext(ctx, "failed to receive matchmaking stream request", "error", err)
				return err
			}

			slog.InfoContext(ctx, "handling matchmaking stream request", "request", request)
			handleMatchmakingRequest(ctx, state, request)
		}
	}

	return nil
}

type MatchmakingID struct {
	UserID int64             `json:"userID"`
	Mode   gamedata.GameMode `json:"mode"`
}

type matchmakingStreamState struct {
	// immutable constructs
	service      *matchmaker.MatchmakerService
	responseChan chan matchmaker.MatchResponse

	// mutatable constructs
	matchmakeID atomic.Pointer[MatchmakingID]
}

func newMatchmakingStreamState(service *matchmaker.MatchmakerService, responseChan chan matchmaker.MatchResponse) *matchmakingStreamState {
	return &matchmakingStreamState{service: service, responseChan: responseChan}
}

func (state *matchmakingStreamState) setMatchmakingID(matchmakingID MatchmakingID) {
	state.matchmakeID.Store(&matchmakingID)
}

func (state *matchmakingStreamState) getMatchmakingID() MatchmakingID {
	matchmakingID := state.matchmakeID.Load()
	return *matchmakingID
}

func handleMatchmakingRequest(ctx context.Context, state *matchmakingStreamState, request *rpc.MatchmakingRequest) {
	switch value := request.Value.(type) {
	case *rpc.MatchmakingRequest_Begin:
		inputMode, err := enum.Parse(value.Begin.Mode, gamedata.GameModeEnums)
		if err != nil {
			slog.ErrorContext(ctx, "invalid mode in recv begin request, dropping message", "error", err)
			return
		}

		matchmakingID := &MatchmakingID{UserID: value.Begin.UserId, Mode: inputMode}
		beginRequest := matchmaker.MatchmakingRequest{
			Kind:         matchmaker.BeginMatchRequest,
			UserID:       matchmakingID.UserID,
			Mode:         matchmakingID.Mode,
			UserElo:      value.Begin.UserElo,
			ResponseChan: state.responseChan,
		}
		state.service.SendMatchRequest(beginRequest)

		state.matchmakeID.Store(matchmakingID)
	case *rpc.MatchmakingRequest_Confirm:
		if value.Confirm.Accept {
			matchmakingID := state.getMatchmakingID()
			confirmRequest := matchmaker.MatchmakingRequest{
				Kind:   matchmaker.ConfirmMatchRequest,
				UserID: matchmakingID.UserID,
				Mode:   matchmakingID.Mode,
			}
			state.service.SendMatchRequest(confirmRequest)
		}
	}
}

func mapMatchmakingResponse(matchResponse matchmaker.MatchResponse) *rpc.MatchmakingResponse {
	response := &rpc.MatchmakingResponse{}

	switch matchResponse.Kind {
	case matchmaker.MatchResponseJoin:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Join{
			Join: &rpc.MatchmakingJoinResponse{},
		}}
	case matchmaker.MatchResponseProposal:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Proposal{
			Proposal: &rpc.MatchmakingProposalResponse{
				UserOneId: matchResponse.UserOneID,
				UserTwoId: matchResponse.UserTwoID,
			},
		}}
	case matchmaker.MatchResponseConfirmationAccepted:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Confirm{
			Confirm: &rpc.MatchmakingConfirmResponse{},
		}}
	case matchmaker.MatchResponseCancel:
		response = &rpc.MatchmakingResponse{Value: &rpc.MatchmakingResponse_Cancel{
			Cancel: &rpc.MatchmakingCancelResponse{},
		}}
	}

	return response
}

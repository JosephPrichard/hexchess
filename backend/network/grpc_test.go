package network_test

import (
	"context"
	"hexchess-svc/rpc"
	"hexchess-svc/service"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

var matchmakingResponseAssertionCmpOpts = []cmp.Option{protocmp.Transform()}

type matchmakerStream struct {
	requests      []*rpc.MatchmakingRequest
	wantResponses []*rpc.MatchmakingResponse
	client        rpc.MatchmakerService_MatchmakingStreamClient
}

func TestMatchmakingStream(t *testing.T) {
	tests := []struct {
		name    string
		config  *service.MatchmakerServiceConfig
		streams []matchmakerStream
	}{
		//{
		//	name:   "BeginRequest_OneConfirmRequest",
		//	config: &service.MatchmakerServiceConfig{TickRate: 1 * time.Minute, TickFactor: 1},
		//	streams: []matchmakerStream{
		//		{
		//			requests: []*rpc.MatchmakingRequest{
		//				{
		//					Value: &rpc.MatchmakingRequest_Begin{Begin: &rpc.MatchmakingBeginRequest{UserId: 1, UserElo: 1000, Mode: "CORRESPONDENCE_1"}},
		//				},
		//				{
		//					Value: &rpc.MatchmakingRequest_Confirm{Confirm: &rpc.MatchmakingConfirmRequest{Accept: true}},
		//				},
		//			},
		//			wantResponses: []*rpc.MatchmakingResponse{
		//				{
		//					Value: &rpc.MatchmakingResponse_Join{Join: &rpc.MatchmakingJoinResponse{}},
		//				},
		//			},
		//		},
		//		{
		//			requests: []*rpc.MatchmakingRequest{
		//				{
		//					Value: &rpc.MatchmakingRequest_Begin{Begin: &rpc.MatchmakingBeginRequest{UserId: 2, UserElo: 1500, Mode: "CORRESPONDENCE_1"}},
		//				},
		//			},
		//			wantResponses: []*rpc.MatchmakingResponse{
		//				{
		//					Value: &rpc.MatchmakingResponse_Join{Join: &rpc.MatchmakingJoinResponse{}},
		//				},
		//			},
		//		},
		//	},
		//},
		//{
		//	name:   "BeginRequest_BothConfirmRequest",
		//	config: &service.MatchmakerServiceConfig{TickRate: 1 * time.Microsecond, TickFactor: 1},
		//	streams: []matchmakerStream{
		//		{
		//			requests: []*rpc.MatchmakingRequest{
		//				{
		//					Value: &rpc.MatchmakingRequest_Begin{Begin: &rpc.MatchmakingBeginRequest{UserId: 1, UserElo: 1000, Mode: "CORRESPONDENCE_1"}},
		//				},
		//				{
		//					Value: &rpc.MatchmakingRequest_Confirm{Confirm: &rpc.MatchmakingConfirmRequest{Accept: true}},
		//				},
		//			},
		//			wantResponses: []*rpc.MatchmakingResponse{
		//				{
		//					Value: &rpc.MatchmakingResponse_Join{Join: &rpc.MatchmakingJoinResponse{}},
		//				},
		//				{
		//					Value: &rpc.MatchmakingResponse_Proposal{Proposal: &rpc.MatchmakingProposalResponse{UserOneId: 1, UserTwoId: 2}},
		//				},
		//			},
		//		},
		//		{
		//			requests: []*rpc.MatchmakingRequest{
		//				{
		//					Value: &rpc.MatchmakingRequest_Begin{Begin: &rpc.MatchmakingBeginRequest{UserId: 2, UserElo: 1500, Mode: "CORRESPONDENCE_1"}},
		//				},
		//				{
		//					Value: &rpc.MatchmakingRequest_Confirm{Confirm: &rpc.MatchmakingConfirmRequest{Accept: true}},
		//				},
		//			},
		//			wantResponses: []*rpc.MatchmakingResponse{
		//				{
		//					Value: &rpc.MatchmakingResponse_Join{Join: &rpc.MatchmakingJoinResponse{}},
		//				},
		//				{
		//					Value: &rpc.MatchmakingResponse_Proposal{Proposal: &rpc.MatchmakingProposalResponse{UserOneId: 1, UserTwoId: 2}},
		//				},
		//			},
		//		},
		//	},
		//},
		//{
		//	name:   "BeginRequest_CancelRequest",
		//	config: &service.MatchmakerServiceConfig{TickRate: 1 * time.Minute, TickFactor: 1},
		//	streams: []matchmakerStream{
		//		{
		//			requests: []*rpc.MatchmakingRequest{
		//				{
		//					Value: &rpc.MatchmakingRequest_Begin{Begin: &rpc.MatchmakingBeginRequest{UserId: 1, UserElo: 1000, Mode: "CORRESPONDENCE_1"}},
		//				},
		//			},
		//			wantResponses: []*rpc.MatchmakingResponse{},
		//		},
		//	},
		//},
		//{
		//	name:   "BeginRequest_OneConfirmRequest_OneCancelRequest",
		//	config: &service.MatchmakerServiceConfig{TickRate: 1 * time.Minute, TickFactor: 1},
		//},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			grpcTest := setupGRPCTest(t, grpcTestConfig{matchmakingServerConfig: test.config})
			defer grpcTest.Shutdown()

			ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*250)
			defer cancel()

			for i := range test.streams {
				streamClient, err := rpc.NewMatchmakerServiceClient(grpcTest.client).MatchmakingStream(ctx)
				if err != nil {
					t.Fatalf("failed to create grpc client stream: %v", err)
				}
				test.streams[i].client = streamClient
			}

			for _, stream := range test.streams {
				for _, request := range stream.requests {
					stream.client.Send(request)
				}
			}

			responseTable := getMatchmakerStreamResponses(t, test.streams)

			for i := range responseTable {
				testutil.Equal(t, test.streams[i].wantResponses, responseTable[i], matchmakingResponseAssertionCmpOpts...)
			}
		})
	}
}

func getMatchmakerStreamResponses(t *testing.T, streams []matchmakerStream) [][]*rpc.MatchmakingResponse {
	responseTable := make([][]*rpc.MatchmakingResponse, 0)
	for _, stream := range streams {
		responses := make([]*rpc.MatchmakingResponse, 0)
		for {
			response, err := stream.client.Recv()
			if err != nil {
				if st, ok := status.FromError(err); ok && st.Code() == codes.DeadlineExceeded {
					break
				}
				t.Fatalf("failed to recv from grpc stream: %v", err)
			}
			responses = append(responses, response)
		}
		responseTable = append(responseTable, responses)
	}
	return responseTable
}

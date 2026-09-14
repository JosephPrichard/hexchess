package service

import (
	"hexchess-svc/model"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var cmpOptsMatchmakingState = []cmp.Option{
	cmpopts.IgnoreFields(PendingUser{}, "Ticks", "ResponseChan"),
}

func TestMatchRequests(t *testing.T) {
	tests := []struct {
		name          string
		request       MatchmakingRequest
		wantResponses []MatchResponse
		beforeState   MatchmakerState
		afterState    MatchmakerState
	}{
		{
			name:          "MatchRequestBegin",
			request:       NewMatchRequestBegin(2, model.ModeCorrespondence1, 1000, make(chan MatchResponse, 1)),
			wantResponses: []MatchResponse{},
			beforeState: MatchmakerState{
				PendingUsers: map[int64]PendingUser{
					1: {UserID: 1, UserElo: 900},
					3: {UserID: 3, UserElo: 1500},
				},
				UserEloIdx: []UserEloRecord{
					{UserID: 1, UserElo: 900},
					{UserID: 3, UserElo: 1500},
				},
				MatchProposals: []MatchProposal{},
			},
			afterState: MatchmakerState{
				PendingUsers: map[int64]PendingUser{
					1: {UserID: 1, UserElo: 900},
					2: {UserID: 2, UserElo: 1000},
					3: {UserID: 3, UserElo: 1500},
				},
				UserEloIdx: []UserEloRecord{
					{UserID: 1, UserElo: 900},
					{UserID: 2, UserElo: 1000},
					{UserID: 3, UserElo: 1500},
				},
				MatchProposals: []MatchProposal{},
			},
		},
		{
			name:          "MatchRequestBegin_Duplicate",
			request:       NewMatchRequestConfirmation(1, model.ModeCorrespondence1),
			wantResponses: []MatchResponse{},
			beforeState: MatchmakerState{
				PendingUsers:   map[int64]PendingUser{1: {UserID: 1, UserElo: 1000}},
				MatchProposals: []MatchProposal{},
			},
			afterState: MatchmakerState{
				PendingUsers:   map[int64]PendingUser{1: {UserID: 1, UserElo: 1000}},
				MatchProposals: []MatchProposal{},
			},
		},
		{
			name:          "MatchRequestConfirmation_Waiting",
			request:       NewMatchRequestConfirmation(1, model.ModeCorrespondence1),
			wantResponses: []MatchResponse{},
			beforeState: MatchmakerState{
				PendingUsers: map[int64]PendingUser{},
				MatchProposals: []MatchProposal{
					{
						UserOne:       PendingUser{UserID: 1, UserElo: 1000},
						UserOneAccept: false,
						UserTwo:       PendingUser{UserID: 2, UserElo: 1000},
						UserTwoAccept: false,
					},
				},
			},
			afterState: MatchmakerState{
				PendingUsers: map[int64]PendingUser{},
				MatchProposals: []MatchProposal{
					{
						UserOne:       PendingUser{UserID: 1, UserElo: 1000},
						UserOneAccept: true,
						UserTwo:       PendingUser{UserID: 2, UserElo: 1000},
						UserTwoAccept: false,
					},
				},
			},
		},
		{
			name:    "MatchRequestConfirmation_Complete",
			request: NewMatchRequestConfirmation(2, model.ModeCorrespondence1),
			wantResponses: []MatchResponse{
				{Kind: MatchResponseConfirmation, UserOneID: 1, UserTwoID: 2},
				{}, // close
				{Kind: MatchResponseConfirmation, UserOneID: 1, UserTwoID: 2},
				{}, // close
			},
			beforeState: MatchmakerState{
				PendingUsers: map[int64]PendingUser{},
				MatchProposals: []MatchProposal{
					{
						UserOne:       PendingUser{UserID: 1, UserElo: 1000, ResponseChan: make(chan MatchResponse, 2)},
						UserOneAccept: true,
						UserTwo:       PendingUser{UserID: 2, UserElo: 1000, ResponseChan: make(chan MatchResponse, 2)},
						UserTwoAccept: false,
					},
				},
			},
			afterState: MatchmakerState{
				PendingUsers:   map[int64]PendingUser{},
				MatchProposals: []MatchProposal{},
			},
		},
		{
			name:    "MatchRequestCancel",
			request: NewMatchRequestCancel(1, model.ModeCorrespondence1),
			wantResponses: []MatchResponse{
				{}, // close
				{Kind: MatchResponseCancel},
			},
			beforeState: MatchmakerState{
				PendingUsers: map[int64]PendingUser{},
				MatchProposals: []MatchProposal{
					{
						UserOne:       PendingUser{UserID: 1, UserElo: 1000, ResponseChan: make(chan MatchResponse, 1)},
						UserOneAccept: false,
						UserTwo:       PendingUser{UserID: 2, UserElo: 1000, ResponseChan: make(chan MatchResponse, 1)},
						UserTwoAccept: false,
					},
				},
			},
			afterState: MatchmakerState{
				PendingUsers:   map[int64]PendingUser{2: {UserID: 2, UserElo: 1000}},
				UserEloIdx:     []UserEloRecord{{UserID: 2, UserElo: 1000}},
				MatchProposals: []MatchProposal{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matchmakerService := NewMatchmakerService(&MatchmakerServiceConfig{TickRate: 1 * time.Minute, TickFactor: 1})

			node := matchmakerService.nodeMap[test.request.Mode]
			node.state = test.beforeState

			responseChans := getResponseChans(test.beforeState)

			matchmakerService.SendMatchRequest(test.request)
			matchmakerService.Shutdown()

			testutil.Equal(t, test.afterState, node.state, cmpOptsMatchmakingState...)

			testutil.Equal(t, test.wantResponses, getResponses(responseChans))
		})
	}
}

func getResponseChans(state MatchmakerState) []chan MatchResponse {
	responseChans := make([]chan MatchResponse, 0)
	for _, user := range state.PendingUsers {
		responseChans = append(responseChans, user.ResponseChan)
	}
	for _, proposal := range state.MatchProposals {
		responseChans = append(responseChans, proposal.UserOne.ResponseChan)
		responseChans = append(responseChans, proposal.UserTwo.ResponseChan)
	}
	return responseChans
}

func getResponses(responseChans []chan MatchResponse) []MatchResponse {
	actualResponses := make([]MatchResponse, 0)
	for _, responseChan := range responseChans {
		if responseChan != nil {
			for range cap(responseChan) {
				actualResponses = append(actualResponses, <-responseChan)
			}
		}
	}
	return actualResponses
}

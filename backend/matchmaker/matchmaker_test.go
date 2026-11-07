package matchmaker

import (
	"hexchess-svc/gamedata"
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
		name    string
		request MatchmakingRequest

		// responses expected by all clients (channels) in the beforeState, in deterministic ordering of recipient and send time
		wantResponses []MatchResponse

		// test sets the state to this before executing the request (the precondition)
		beforeState MatchmakerState

		// this is the expected state after we send the request (the postcondition)
		afterState MatchmakerState
	}{
		{
			name:          "MatchRequestBegin",
			request:       MatchmakingRequest{Kind: BeginMatchRequest, UserID: 2, Mode: gamedata.ModeCorrespondence1, UserElo: 1000, ResponseChan: make(chan MatchResponse, 1)},
			wantResponses: []MatchResponse{}, // do not wait for responses (not expecting any)
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
			request:       MatchmakingRequest{Kind: ConfirmMatchRequest, UserID: 1, Mode: gamedata.ModeCorrespondence1},
			wantResponses: []MatchResponse{}, // do not wait for responses (not expecting any)
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
			request:       MatchmakingRequest{Kind: ConfirmMatchRequest, UserID: 1, Mode: gamedata.ModeCorrespondence1},
			wantResponses: []MatchResponse{}, // do not wait for responses (not expecting any)
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
			request: MatchmakingRequest{Kind: ConfirmMatchRequest, UserID: 2, Mode: gamedata.ModeCorrespondence1},
			wantResponses: []MatchResponse{
				{Kind: MatchResponseConfirmationAccepted, UserOneID: 1, UserTwoID: 2},
				{}, // channel close
				{Kind: MatchResponseConfirmationAccepted, UserOneID: 1, UserTwoID: 2},
				{}, // channel close
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
			request: MatchmakingRequest{Kind: CancelMatchRequest, UserID: 1, Mode: gamedata.ModeCorrespondence1},
			wantResponses: []MatchResponse{
				{}, // channel close
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
			ackedRequestChan := make(chan MatchmakingRequest, 1)

			matchmakerService := NewMatchmakerService(MatchmakerServiceConfig{
				TickRate:         1 * time.Minute,
				TickFactor:       1,
				ackedRequestChan: ackedRequestChan,
			})
			defer matchmakerService.Shutdown()

			node := matchmakerService.nodeMap[test.request.Mode]
			node.state = test.beforeState

			responseChans := getResponseChans(node.state)

			matchmakerService.SendMatchRequest(test.request)

			// wait until all responses are received and request is handled (determinism)
			waitHandleRequest(t, ackedRequestChan)
			responses := getResponses(t, responseChans)

			afterState := node.state

			testutil.Equal(t, test.wantResponses, responses)
			testutil.Equal(t, test.afterState, afterState, cmpOptsMatchmakingState...)
		})
	}
}

func waitHandleRequest(t *testing.T, ackedRequestChan chan MatchmakingRequest) {
	select {
	case <-ackedRequestChan:
	case <-t.Context().Done():
		t.Fatalf("waiting on handled request chan timed out: %v", t.Context().Err())
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

func getResponses(t *testing.T, responseChans []chan MatchResponse) []MatchResponse {
	actualResponses := make([]MatchResponse, 0)
	for _, responseChan := range responseChans {
		if responseChan != nil {
			for range cap(responseChan) {
				select {
				case response := <-responseChan:
					actualResponses = append(actualResponses, response)
				case <-t.Context().Done():
					t.Fatalf("waiting on response chans timed out: %v", t.Context().Err())
				}
			}
		}
	}
	return actualResponses
}

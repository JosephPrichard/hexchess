package service

import (
	"hexchess-svc/model"
	"slices"
	"time"
)

type MatchmakerService struct {
	modeNodeMap map[model.GameMode]*serviceNode
}

func NewMatchmakerService() *MatchmakerService {
	modeNodeMap := map[model.GameMode]*serviceNode{}

	for _, mode := range model.GameModeEnums {
		node := &serviceNode{
			requestChan: make(chan MatchmakingRequest),
			stopChan:    make(chan struct{}),
		}
		go node.Run()
		modeNodeMap[mode] = node
	}

	return &MatchmakerService{modeNodeMap: modeNodeMap}
}

type MatchRequestKind int

const (
	MatchRequestBegin MatchRequestKind = iota
	MatchRequestConfirmation
	MatchRequestCancel
)

type MatchmakingRequest struct {
	Kind         MatchRequestKind
	MatchmakeID  MatchmakeID
	UserElo      float64
	ResponseChan chan MatchResponse
}

type MatchmakeID struct {
	UserID int64
	Mode   model.GameMode
}

func NewMatchRequestBegin(matchmakeID MatchmakeID, userElo float64, responseChan chan MatchResponse) MatchmakingRequest {
	return MatchmakingRequest{
		Kind:         MatchRequestBegin,
		MatchmakeID:  matchmakeID,
		UserElo:      userElo,
		ResponseChan: responseChan,
	}
}

func NewMatchRequestConfirmation(matchmakeID MatchmakeID) MatchmakingRequest {
	return MatchmakingRequest{Kind: MatchRequestBegin, MatchmakeID: matchmakeID}
}

func NewMatchRequestCancel(matchmakeID MatchmakeID) MatchmakingRequest {
	return MatchmakingRequest{Kind: MatchRequestCancel, MatchmakeID: matchmakeID}
}

func (services *MatchmakerService) SendMatchRequest(request MatchmakingRequest) {
	node := services.modeNodeMap[request.MatchmakeID.Mode]
	node.requestChan <- request
}

type MatchResponseKind int

const (
	MatchResponseProposal     MatchResponseKind = iota // sent in cases where the server is proposing a potential match
	MatchResponseConfirmation                          // sent when both users have accepted the match
)

type MatchResponse struct {
	Kind      MatchResponseKind
	UserTwoID int64
	UserOneID int64
}

type serviceNode struct {
	requestChan chan MatchmakingRequest
	stopChan    chan struct{}
}

type runState struct {
	pendingUsers   []PendingUser
	matchProposals []MatchProposal
}

type PendingUser struct {
	UserID       int64
	UserElo      float64
	Ticks        int64 // number ticks the user has remained in the matchmaking pool
	ResponseChan chan MatchResponse
}

type MatchProposal struct {
	ResponseChan  chan MatchResponse
	UserOne       PendingUser
	UserOneAccept bool
	UserTwo       PendingUser
	UserTwoAccept bool
}

const TickRate = time.Millisecond * 500

func (node *serviceNode) Run() {
	ticker := time.NewTicker(TickRate)
	state := &runState{pendingUsers: make([]PendingUser, 0)}

	for {
		select {
		case <-ticker.C:
			tick(state)
		case request := <-node.requestChan:
			handleRequest(state, request)
		case <-node.stopChan:
			return
		}
	}
}

func tick(state *runState) {

}

func handleRequest(state *runState, request MatchmakingRequest) {
	switch request.Kind {
	case MatchRequestBegin:
		handleBeginRequest(state, request)
	case MatchRequestConfirmation:
		handleConfirmRequest(state, request)
	case MatchRequestCancel:
		handleCancelRequest(state, request)
	}
}

func handleBeginRequest(state *runState, request MatchmakingRequest) {
	state.pendingUsers = append(state.pendingUsers, PendingUser{
		ResponseChan: request.ResponseChan,
		UserID:       request.MatchmakeID.UserID,
		UserElo:      request.UserElo,
	})
}

func handleConfirmRequest(state *runState, request MatchmakingRequest) {
	proposalIndex := slices.IndexFunc(state.matchProposals, func(user MatchProposal) bool {
		return user.UserOne.UserID == request.MatchmakeID.UserID || user.UserTwo.UserID == request.MatchmakeID.UserID
	})
	matchProposal := &state.matchProposals[proposalIndex]

	switch request.MatchmakeID.UserID {
	case matchProposal.UserOne.UserID:
		matchProposal.UserOneAccept = true
	case matchProposal.UserTwo.UserID:
		matchProposal.UserTwoAccept = true
	}

	if matchProposal.UserOneAccept && matchProposal.UserTwoAccept {
		matchConfirmResponse := MatchResponse{
			Kind:      MatchResponseConfirmation,
			UserOneID: matchProposal.UserOne.UserID,
			UserTwoID: matchProposal.UserTwo.UserID,
		}
		matchProposal.UserOne.ResponseChan <- matchConfirmResponse
		matchProposal.UserTwo.ResponseChan <- matchConfirmResponse
	}
}

func handleCancelRequest(state *runState, request MatchmakingRequest) {
	// note(Joseph): remove any
	state.pendingUsers = slices.DeleteFunc(state.pendingUsers, func(user PendingUser) bool {
		return user.UserID == request.MatchmakeID.UserID
	})

	proposalIndex := slices.IndexFunc(state.matchProposals, func(user MatchProposal) bool {
		return user.UserOne.UserID == request.MatchmakeID.UserID || user.UserTwo.UserID == request.MatchmakeID.UserID
	})
	matchProposal := &state.matchProposals[proposalIndex]
}

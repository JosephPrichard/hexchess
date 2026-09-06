package matchmaking

import (
	"hexchess-svc/model"
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
)

type MatchmakingRequest struct {
	Kind         MatchRequestKind
	Mode         model.GameMode
	UserID       int64
	UserElo      float64
	ResponseChan chan MatchResponse
}

func (services *MatchmakerService) SendMatchRequest(request MatchmakingRequest) {
	node := services.modeNodeMap[request.Mode]
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

type User struct {
	UserID  int64
	UserElo float64
	Ticks   int64 // number ticks the user has remained in the matchmaking pool
}

type PendingUser struct {
	ResponseChan chan MatchResponse
	User
}

type MatchProposal struct {
	ResponseChan chan MatchResponse
	UserOne      User
	UserTwo      User
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
		state.pendingUsers = append(state.pendingUsers, PendingUser{
			ResponseChan: request.ResponseChan,
			User: User{
				UserID:  request.UserID,
				UserElo: request.UserElo,
			},
		})
	case MatchRequestConfirmation:

	}
}

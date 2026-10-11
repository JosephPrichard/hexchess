package service

import (
	"hexchess-svc/model"
	"hexchess-svc/utils/perf"
	"log/slog"
	"slices"
	"sort"
	"time"
)

type MatchRequestKind int

const (
	BeginMatchRequest MatchRequestKind = iota
	ConfirmMatchRequest
	CancelMatchRequest
)

type MatchmakingRequest struct {
	Kind         MatchRequestKind   `json:"kind"`
	UserID       int64              `json:"userID"`
	Mode         model.GameMode     `json:"mode"`
	UserElo      float64            `json:"userElo"`
	ResponseChan chan MatchResponse `json:"responseChan"`
}

type MatchResponseKind int

const (
	// MatchResponseJoin sent when a user who requests a match is moved into the pending users pool
	MatchResponseJoin MatchResponseKind = iota
	// MatchResponseProposal sent in cases where the server is proposing a potential match
	MatchResponseProposal
	// MatchResponseConfirmation sent when both users have accepted the match
	MatchResponseConfirmation
	// MatchResponseCancel sent when another player cancels a potential match rather than accepting
	MatchResponseCancel
)

type MatchResponse struct {
	Kind      MatchResponseKind
	UserTwoID int64
	UserOneID int64
}

type MatchmakingNode int

type MatchmakerService struct {
	nodeMap map[model.GameMode]*matchmakerNode
}

type MatchmakerServiceConfig struct {
	TickRate   time.Duration `json:"tickRate"`
	TickFactor int64         `json:"tickFactor"`
}

var DefaultMatchmakerServiceConfig = MatchmakerServiceConfig{
	TickRate:   time.Millisecond * 500,
	TickFactor: 10, // every tick a pendingUser is not provided a match - increase the (+-) elo range by this value
}

func NewMatchmakerService(cfg *MatchmakerServiceConfig) *MatchmakerService {
	config := DefaultMatchmakerServiceConfig
	if cfg != nil {
		config = *cfg
	}

	nodeMap := map[model.GameMode]*matchmakerNode{}

	for _, mode := range model.GameModeEnums {
		matchmaker := &matchmakerNode{
			config: config,

			requestChan: make(chan MatchmakingRequest),
			stopChan:    make(chan struct{}),

			state: MatchmakerState{
				PendingUsers:   make(map[int64]PendingUser),
				UserEloIdx:     make([]UserEloRecord, 0),
				MatchProposals: make([]MatchProposal, 0),
			},
		}
		go matchmaker.Run()
		nodeMap[mode] = matchmaker
	}

	return &MatchmakerService{nodeMap: nodeMap}
}

func (services *MatchmakerService) SendMatchRequest(request MatchmakingRequest) {
	node := services.nodeMap[request.Mode]
	if node == nil {
		slog.Error("matchmaker service node does not exist for mode", "request", request)
		return
	}
	node.requestChan <- request
}

func (services *MatchmakerService) Shutdown() {
	for _, node := range services.nodeMap {
		node.Shutdown()
	}
}

type matchmakerNode struct {
	config MatchmakerServiceConfig

	// CSP - communicate with a matchmaker instance by using channels
	requestChan chan MatchmakingRequest
	stopChan    chan struct{}

	state MatchmakerState
}

type MatchmakerState struct {
	PendingUsers map[int64]PendingUser `json:"pendingUsers"` // userID->user map structure storing users that are waiting matches
	UserEloIdx   []UserEloRecord       `json:"userEloIdx"`   // indexing data-structure for elo range query lookups on user data

	MatchProposals []MatchProposal `json:"matchProposals"` // working list of matches proposed to user participants

	PotentialProposals []potentialProposal
	PairedUsers        map[int64]struct{}
}

type UserEloRecord struct {
	UserID  int64
	UserElo float64
}

type PendingUser struct {
	UserID       int64
	UserElo      float64
	Ticks        int64 // number of ticks the user has remained in the matchmaking pool
	ResponseChan chan MatchResponse
}

type MatchProposal struct {
	UserOne       PendingUser
	UserTwo       PendingUser
	UserOneAccept bool
	UserTwoAccept bool
}

type potentialProposal struct {
	userOneID int64
	userTwoID int64
}

func (node *matchmakerNode) Run() {
	ticker := time.NewTicker(node.config.TickRate)
	for {
		select {
		case <-ticker.C:
			node.tick()
		case request := <-node.requestChan:
			node.handleRequest(request)
		case <-node.stopChan:
			return
		}
	}
}

func (node *matchmakerNode) Shutdown() {
	close(node.stopChan)
}

func (node *matchmakerNode) tick() {
	if len(node.state.PendingUsers) == 0 && len(node.state.MatchProposals) == 0 {
		return
	}
	defer perf.New().Log()

	// transfer any matching pendingUsers into matchProposals based on range (elo-tick*TickFactor,elo+tick*TickFactor)
	node.findPotentialProposals()

	// remove any users in potentialProposals from pendingUsers and transfer them to matchProposals
	for _, proposal := range node.state.PotentialProposals {
		node.appendMatchProposal(MatchProposal{
			UserOne:       node.state.PendingUsers[proposal.userOneID],
			UserTwo:       node.state.PendingUsers[proposal.userTwoID],
			UserOneAccept: false,
			UserTwoAccept: false,
		})
	}
	node.deletePendingUsers(node.state.PairedUsers)

	// update tick state of all pendingUsers
	for _, record := range node.state.UserEloIdx {
		pendingUser := node.state.PendingUsers[record.UserID]
		pendingUser.Ticks++
		node.state.PendingUsers[record.UserID] = pendingUser
	}
}

func (node *matchmakerNode) findPotentialProposals() {
	// reusing internal datastructures - need to clear out stale data first
	if node.state.PotentialProposals != nil {
		node.state.PotentialProposals = node.state.PotentialProposals[:0]
	} else {
		node.state.PotentialProposals = make([]potentialProposal, 0)
	}
	if node.state.PairedUsers != nil {
		clear(node.state.PairedUsers)
	} else {
		node.state.PairedUsers = make(map[int64]struct{})
	}

	potentialProposals := node.state.PotentialProposals
	pairedUsers := node.state.PairedUsers

	for currIdx := range node.state.UserEloIdx {
		currUserID := node.state.UserEloIdx[currIdx].UserID
		currPendingUser := node.state.PendingUsers[currUserID]

		_, isCurrUserPaired := pairedUsers[currUserID]
		if isCurrUserPaired {
			continue
		}

		// find the closest right-most pendingUser and pair it into a potential proposal if possible
		for potentialIdx := currIdx; potentialIdx < len(node.state.UserEloIdx); potentialIdx++ {
			potentialUserID := node.state.UserEloIdx[potentialIdx].UserID
			potentialPendingUser := node.state.PendingUsers[potentialUserID]

			_, isPotentialUserPaired := pairedUsers[potentialUserID]
			if isPotentialUserPaired {
				continue
			}

			doesCurrUserIntersect, doesPotentialUserIntersect := node.pendingUsersIntersect(&currPendingUser, &potentialPendingUser)

			bothUsersIntersect := doesCurrUserIntersect && doesPotentialUserIntersect
			if bothUsersIntersect {
				potentialProposals = append(potentialProposals, potentialProposal{
					userOneID: currPendingUser.UserID,
					userTwoID: potentialPendingUser.UserID,
				})

				pairedUsers[currUserID] = struct{}{}
				pairedUsers[potentialUserID] = struct{}{}

				break
			}
			if !doesPotentialUserIntersect {
				break
			}
		}
	}
}

func isInRange(potentialUser *PendingUser, beginElo float64, endElo float64) bool {
	return potentialUser.UserElo >= beginElo && potentialUser.UserElo <= endElo
}

func (node *matchmakerNode) pendingUsersIntersect(currPendingUser *PendingUser, potentialPendingUser *PendingUser) (bool, bool) {
	currBeginElo, currEndElo := node.getEloRange(currPendingUser)
	potentialBeginElo, potentialEndElo := node.getEloRange(potentialPendingUser)

	doesCurrUserIntersect := isInRange(currPendingUser, potentialBeginElo, potentialEndElo)
	doesPotentialUserIntersect := isInRange(potentialPendingUser, currBeginElo, currEndElo)

	return doesCurrUserIntersect, doesPotentialUserIntersect
}

func (node *matchmakerNode) getEloRange(pendingUser *PendingUser) (float64, float64) {
	eloOffset := float64(pendingUser.Ticks * node.config.TickFactor)
	beginElo := pendingUser.UserElo - eloOffset
	endElo := pendingUser.UserElo + eloOffset
	return beginElo, endElo
}

func (node *matchmakerNode) handleRequest(request MatchmakingRequest) {
	switch request.Kind {
	case BeginMatchRequest:
		node.handleBeginRequest(request)
	case ConfirmMatchRequest:
		node.handleConfirmRequest(request)
	case CancelMatchRequest:
		node.handleCancelRequest(request)
	}
}

func (node *matchmakerNode) handleBeginRequest(request MatchmakingRequest) {
	defer perf.New().Log()

	node.insertPendingUser(PendingUser{
		ResponseChan: request.ResponseChan,
		UserID:       request.UserID,
		UserElo:      request.UserElo,
	})

	request.ResponseChan <- MatchResponse{Kind: MatchResponseJoin}
}

func (node *matchmakerNode) handleConfirmRequest(request MatchmakingRequest) {
	defer perf.New().Log()

	proposal, index := node.getMatchProposal(request.UserID)
	if index == -1 {
		slog.Warn("user does not have a match to confirm - skipping", "userID", request.UserID)
		return
	}

	// mark matchProposal as accepted and reinsert
	switch request.UserID {
	case proposal.UserOne.UserID:
		proposal.UserOneAccept = true
	case proposal.UserTwo.UserID:
		proposal.UserTwoAccept = true
	}
	node.setMatchProposal(index, proposal)

	// when matchProposal is accepted, communicate the confirmation, close the channels, and delete the proposal
	isProposalAccepted := proposal.UserOneAccept && proposal.UserTwoAccept
	if isProposalAccepted {
		matchConfirmResponse := MatchResponse{Kind: MatchResponseConfirmation, UserOneID: proposal.UserOne.UserID, UserTwoID: proposal.UserTwo.UserID}

		proposal.UserOne.ResponseChan <- matchConfirmResponse
		proposal.UserTwo.ResponseChan <- matchConfirmResponse
		close(proposal.UserOne.ResponseChan)
		close(proposal.UserTwo.ResponseChan)

		node.deleteMatchProposal(index)
	}
}

func (node *matchmakerNode) handleCancelRequest(request MatchmakingRequest) {
	defer perf.New().Log()

	node.deletePendingUser(request.UserID)

	proposal, index := node.getMatchProposal(request.UserID)
	if index == -1 {
		slog.Warn("user does not have a match to cancel - skipping", "userID", request.UserID)
		return
	}

	var requestingUser, otherUser PendingUser
	switch request.UserID {
	case proposal.UserOne.UserID:
		requestingUser, otherUser = proposal.UserOne, proposal.UserTwo
	case proposal.UserTwo.UserID:
		requestingUser, otherUser = proposal.UserTwo, proposal.UserOne
	}

	node.deleteMatchProposal(index)
	node.insertPendingUser(otherUser)

	matchCancelResponse := MatchResponse{Kind: MatchResponseCancel}
	close(requestingUser.ResponseChan)
	otherUser.ResponseChan <- matchCancelResponse
}

func (node *matchmakerNode) insertPendingUser(pendingUser PendingUser) {
	state := &node.state

	_, exists := state.PendingUsers[pendingUser.UserID]
	if exists {
		return
	}

	// insert into central record for pending user data
	state.PendingUsers[pendingUser.UserID] = pendingUser

	// insert record into sorted user-by-elo index
	userEloRecord := UserEloRecord{UserID: pendingUser.UserID, UserElo: pendingUser.UserElo}

	insertionIndex := sort.Search(len(state.UserEloIdx), func(i int) bool {
		return state.UserEloIdx[i].UserElo >= pendingUser.UserElo
	})
	if insertionIndex >= 0 || insertionIndex < len(state.PendingUsers) {
		state.UserEloIdx = slices.Insert(state.UserEloIdx, insertionIndex, userEloRecord)
	} else {
		state.UserEloIdx = append(state.UserEloIdx, userEloRecord)
	}
}

func (node *matchmakerNode) getMatchProposal(userID int64) (proposal MatchProposal, index int) {
	index = slices.IndexFunc(node.state.MatchProposals, func(proposal MatchProposal) bool {
		return userID == proposal.UserOne.UserID || userID == proposal.UserTwo.UserID
	})
	if index >= 0 {
		proposal = node.state.MatchProposals[index]
	}
	return
}

func (node *matchmakerNode) setMatchProposal(index int, matchProposal MatchProposal) {
	node.state.MatchProposals[index] = matchProposal
}

func (node *matchmakerNode) deletePendingUser(userID int64) {
	// delete from central record for pending user data
	delete(node.state.PendingUsers, userID)

	// delete record from sorted user-by-elo index
	node.state.UserEloIdx = slices.DeleteFunc(node.state.UserEloIdx, func(user UserEloRecord) bool {
		return user.UserID == userID
	})
}

func (node *matchmakerNode) deletePendingUsers(userIDs map[int64]struct{}) {
	// delete from central record for pending user data
	for userID := range userIDs {
		delete(node.state.PendingUsers, userID)
	}

	// delete record from sorted user-by-elo index
	node.state.UserEloIdx = slices.DeleteFunc(node.state.UserEloIdx, func(user UserEloRecord) bool {
		_, containsUserID := userIDs[user.UserID]
		return containsUserID
	})
}

func (node *matchmakerNode) appendMatchProposal(matchProposal MatchProposal) {
	node.state.MatchProposals = append(node.state.MatchProposals, matchProposal)
}

func (node *matchmakerNode) deleteMatchProposal(proposalIndex int) {
	node.state.MatchProposals = slices.Delete(node.state.MatchProposals, proposalIndex, proposalIndex+1)
}

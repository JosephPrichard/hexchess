package svc

import (
	"cmp"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/model"
	"math"
	"slices"
)

type FirstMatchParticipant struct {
	UserID int64
	Elo    pgtype.Float8
}

type FirstMatchmakingRequest struct {
	Ruleset      model.TournamentRuleset
	Mode         model.GameMode
	Participants []FirstMatchParticipant
	TotalRounds  int32
}

type MatchmakingResponse struct {
	NextMatches    []model.MatchCreation
	TotalRounds    int32 // RoundRobin and Swiss calculate total rounds during matchmaking rather than using already existing rounds to validate
	NextStatus     model.TournamentStatus
	NextMatchRound int32
	WinnerID       int64
	Tiebreaker     TieBreakerKind
}

type TieBreakerKind int

const (
	TiebreakerNone TieBreakerKind = iota
	TiebreakerByElo
	TiebreakerBySonnebornBerger
)

type CompletedPrevMatch struct {
	Round    int32
	WhiteID  int64
	BlackID  int64
	WhiteElo float64
	BlackElo float64
	Result   model.ReplayResult
}

func getKnockoutWinnerID(match CompletedPrevMatch) (int64, TieBreakerKind) {
	switch match.Result {
	case model.WhiteWin:
		return match.WhiteID, TiebreakerNone
	case model.BlackWin:
		return match.BlackID, TiebreakerNone
	// tiebreaker for draw uses the player with the highest elo, with white as a last case scenario
	case model.Draw:
		if match.WhiteElo >= match.BlackElo {
			return match.WhiteID, TiebreakerByElo
		} else {
			return match.BlackID, TiebreakerByElo
		}
	default:
		panic(fmt.Sprintf("unknown match result %s", match.Result))
	}
}

func withoutTiebreaker(u int64, _ TieBreakerKind) int64 {
	return u
}

func calcRoundRobinTournamentRounds(participantCount int) int32 {
	return int32((participantCount * (participantCount - 1)) / 2)
}

func calcSwissTournamentRounds(participantCount int) int32 {
	return int32(math.Log2(float64(participantCount)))
}

// knockoutParticipantsAtRound returns the number of elements at a given depth.
// Each node holds 2 elements. At depth d, there are 2^(N-d) nodes.
func knockoutParticipantsAtRound(maxDepth, depth int) int { return 1 << (maxDepth - depth + 1) }

// knockoutMatchesAtRound returns the number of nodes at a given depth.
// Root (depth N) has 1 node; each level down doubles the count.
func knockoutMatchesAtRound(maxDepth, depth int) int { return 1 << (maxDepth - depth) }

type MatchCountErrKind int

const (
	ParticipantCountErrKind = iota
	ParticipantParityErrKind
	MatchParityErrKind
)

type MatchCountError struct {
	Kind      MatchCountErrKind
	WantCount int
	GotCount  int
}

func (e MatchCountError) Error() string {
	switch e.Kind {
	case ParticipantCountErrKind:
		return fmt.Sprintf("tournament requires %v participants, got: %d", e.WantCount, e.GotCount)
	case ParticipantParityErrKind:
		return fmt.Sprintf("tournament requires an even number of participants, got: %d", e.GotCount)
	case MatchParityErrKind:
		return fmt.Sprintf("tournament requires an even number of matches, got: %d", e.GotCount)
	default:
		return fmt.Sprintf("unknown match participant count error kind %d", e.Kind)
	}
}

func makeMatchesLinearly(participants []FirstMatchParticipant, gameMode model.GameMode) []model.MatchCreation {
	// invariant: participant count is always even (`elementsAtFirstDepth` always returns even)
	if len(participants)%2 != 0 {
		// assert rather than return an error because this property is statically encoded into the `ElementsAtFirstDepth` algorithm
		panic(fmt.Sprintf("participant count %+v is not even", participants))
	}
	var matches []model.MatchCreation
	for i := 0; i+1 < len(participants); i += 2 {
		matches = append(matches, model.MatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  participants[i].UserID,
			BlackID:  participants[i+1].UserID,
		})
	}
	return matches
}

func makeMatchesCrissCrossElos(participants []FirstMatchParticipant, gameMode model.GameMode) []model.MatchCreation {
	// sort participants by elo.
	slices.SortFunc(participants, func(a, b FirstMatchParticipant) int {
		if n := cmp.Compare(model.DefaultUserElo(b.Elo), model.DefaultUserElo(a.Elo)); n != 0 {
			return n
		}
		return cmp.Compare(a.UserID, b.UserID)
	})

	var matches []model.MatchCreation
	low := 0
	high := len(participants) - 1
	for low < high {
		matches = append(matches, model.MatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  participants[low].UserID,
			BlackID:  participants[high].UserID,
		})
		low++
		high--
	}

	// invariant: always converges on a different player (so each player gets a match)
	if low == high {
		// assert rather than return an error because this property is statically encoded into the this algorithm
		panic(fmt.Sprintf("participant count %+v is not even", participants))
	}
	return matches
}

func MakeFirstMatches(request FirstMatchmakingRequest) (MatchmakingResponse, error) {
	participantCount := len(request.Participants)

	var matches []model.MatchCreation
	totalRounds := request.TotalRounds

	switch request.Ruleset {
	case model.TournamentKnockout:
		// invariant: matches are devided by two each time and stop at 1, we need to start at the expected power of 2
		wantRoundCount := knockoutParticipantsAtRound(int(totalRounds), 1)
		if participantCount != wantRoundCount {
			return MatchmakingResponse{}, MatchCountError{Kind: ParticipantCountErrKind, WantCount: wantRoundCount, GotCount: participantCount}
		}
	case model.TournamentRoundRobin, model.TournamentSwiss:
		// invariant: as long as we can match each player with another player, we can start the tournament
		if participantCount%2 != 0 {
			return MatchmakingResponse{}, MatchCountError{Kind: ParticipantParityErrKind, GotCount: participantCount}
		}
	}

	switch request.Ruleset {
	case model.TournamentKnockout:
		matches = makeMatchesLinearly(request.Participants, request.Mode)
		wantRoundCount := knockoutMatchesAtRound(int(totalRounds), 1)
		if len(matches) != wantRoundCount {
			panic(fmt.Sprintf("expected %d matches, got %d", wantRoundCount, len(matches)))
		}
	case model.TournamentRoundRobin:
		matches = makeMatchesLinearly(request.Participants, request.Mode)
		totalRounds = calcRoundRobinTournamentRounds(participantCount)
	case model.TournamentSwiss:
		// a swiss tournament matches the worst players with the best players
		matches = makeMatchesCrissCrossElos(request.Participants, request.Mode)
		totalRounds = calcSwissTournamentRounds(participantCount)
	default:
		return MatchmakingResponse{}, fmt.Errorf("unknown tournament ruleset %s", request.Ruleset)
	}

	return MatchmakingResponse{NextMatches: matches, NextStatus: model.TournamentInProgress, NextMatchRound: 1, TotalRounds: totalRounds}, nil
}

type MatchmakingRequest struct {
	Ruleset     model.TournamentRuleset
	Matches     []CompletedPrevMatch
	GameMode    model.GameMode
	TotalRounds int32
}

func getPrevRoundMatches(matches []CompletedPrevMatch) []CompletedPrevMatch {
	prevMatchRound := matches[len(matches)-1].Round
	var prevRoundMatches []CompletedPrevMatch
	for _, match := range matches {
		if match.Round == prevMatchRound {
			prevRoundMatches = append(prevRoundMatches, match)
		}
	}
	return prevRoundMatches
}

const WinnerIDNone = int64(0)

func DoMatchmaking(request MatchmakingRequest) (MatchmakingResponse, error) {
	// validation: a tournament must have matches to do matchmaking
	if len(request.Matches) == 0 {
		return MatchmakingResponse{}, ErrEmptyMatchesTournament
	}

	gameMode := request.GameMode
	allMatches := request.Matches

	var nextMatches []model.MatchCreation

	switch request.Ruleset {
	case model.TournamentKnockout:
		if len(allMatches)%2 != 0 {
			return MatchmakingResponse{}, MatchCountError{Kind: MatchParityErrKind, GotCount: len(allMatches)}
		}
		nextMatches = DoKnockoutMatchmaking(allMatches, gameMode)
	case model.TournamentRoundRobin:
		nextMatches = DoRoundRobinMatchmaking(allMatches, gameMode)
	case model.TournamentSwiss:
		nextMatches = DoSwissMatchmaking(allMatches, gameMode)
	default:
		return MatchmakingResponse{}, fmt.Errorf("unknown tournament ruleset %s", request.Ruleset)
	}

	prevMatchRound := allMatches[len(allMatches)-1].Round
	nextMatchRound := prevMatchRound + 1

	nextStatus := model.TournamentInProgress
	winnerID := WinnerIDNone // defaults to no winner
	tiebreaker := TiebreakerNone

	if nextMatchRound > request.TotalRounds {
		nextStatus = model.TournamentFinished
		winnerID, tiebreaker = findTournamentWinner(request.Ruleset, request.Matches)
	}

	return MatchmakingResponse{
		NextMatches:    nextMatches,
		NextStatus:     nextStatus,
		NextMatchRound: nextMatchRound,
		TotalRounds:    request.TotalRounds,
		WinnerID:       winnerID,
		Tiebreaker:     tiebreaker,
	}, nil
}

func DoKnockoutMatchmaking(allMatches []CompletedPrevMatch, gameMode model.GameMode) []model.MatchCreation {
	var nextMatches []model.MatchCreation

	prevRoundMatches := getPrevRoundMatches(allMatches)

	for i := 0; i+1 < len(prevRoundMatches); i += 2 {
		matchOne := prevRoundMatches[i]
		matchTwo := prevRoundMatches[i+1]
		nextMatches = append(nextMatches, model.MatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  withoutTiebreaker(getKnockoutWinnerID(matchOne)),
			BlackID:  withoutTiebreaker(getKnockoutWinnerID(matchTwo)),
		})
	}

	if len(nextMatches) != len(prevRoundMatches)/2 {
		panic(fmt.Sprintf("expected %d next matches, got %d", len(prevRoundMatches)/2, len(nextMatches)))
	}

	return nextMatches
}

func DoRoundRobinMatchmaking(allMatches []CompletedPrevMatch, gameMode model.GameMode) []model.MatchCreation {
	var nextMatches []model.MatchCreation

	prevRoundMatches := getPrevRoundMatches(allMatches)

	for i := range len(prevRoundMatches) {
		prevMatch := prevRoundMatches[i]

		var nextWhiteID int64
		var nextBlackID int64

		if i == 0 {
			// case (first match): nextWhiteID acts as an 'anchor' (only player that does not change)
			nextWhiteID = prevMatch.WhiteID
			nextBlackID = prevRoundMatches[len(prevRoundMatches)-1].BlackID
		} else {
			// case (other match): take nextWhiteID from previous match, shift previous nextWhiteID into next nextBlackID
			nextWhiteID = prevRoundMatches[i-1].BlackID
			nextBlackID = prevMatch.WhiteID
		}

		nextMatches = append(nextMatches, model.MatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  nextWhiteID,
			BlackID:  nextBlackID,
		})
	}

	return nextMatches
}

func DoSwissMatchmaking(allMatches []CompletedPrevMatch, gameMode model.GameMode) []model.MatchCreation {
	var nextMatches []model.MatchCreation

	swissScoresTable := makeSwissTable(allMatches)

	// collect and reverse sort match participants by swiss score
	var participantIDs []int64
	for userID, _ := range swissScoresTable {
		participantIDs = append(participantIDs, userID)
	}
	slices.SortFunc(participantIDs, func(a, b int64) int {
		if c := cmp.Compare(swissScoresTable[b], swissScoresTable[a]); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})

	for i := 0; i+1 < len(participantIDs); i += 2 {
		nextMatches = append(nextMatches, model.MatchCreation{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  participantIDs[i],
			BlackID:  participantIDs[i+1],
		})
	}

	return nextMatches
}

func findScoringTableWinners[Score cmp.Ordered](scoreTable map[int64]Score, skip func(int64) bool) []int64 {
	var highestScore Score
	var winnerIDs []int64

	for userID, score := range scoreTable {
		if skip(userID) {
			continue
		}
		if score == highestScore {
			winnerIDs = append(winnerIDs, userID)
		} else if score > highestScore {
			winnerIDs = winnerIDs[:0]
			winnerIDs = append(winnerIDs, userID)
			highestScore = score
		}
	}

	if len(winnerIDs) == 0 {
		// the only way this can pass is if the table is empty because the match list that produced the table was empty
		// a tournament with no matches is impossible since it should have never passed the IN_PROGRESS status
		panic("expected scoring table to produce at least one winner, got none")
	}
	return winnerIDs
}

func findTournamentWinner(ruleset model.TournamentRuleset, allMatches []CompletedPrevMatch) (int64, TieBreakerKind) {
	switch ruleset {
	case model.TournamentKnockout:
		// winner of the tournament is the player left standing
		return getKnockoutWinnerID(allMatches[len(allMatches)-1])
	case model.TournamentRoundRobin:
		winCountTable := makeWinCountTable(allMatches)

		// standard: player with the most total wins will win the tournament
		totalWinCheckWinnerIDs := findScoringTableWinners(winCountTable, func(int64) bool { return false })
		if len(totalWinCheckWinnerIDs) == 1 {
			return totalWinCheckWinnerIDs[0], TiebreakerNone
		}

		sonnebornTable := makeSonnebornTable(allMatches)

		// tiebreaker: use the Sonneborn-Berger score for each tied winner, largest wins
		sonnebornWinnerIDs := findScoringTableWinners(sonnebornTable, func(userID int64) bool { return !slices.Contains(totalWinCheckWinnerIDs, userID) })

		return sonnebornWinnerIDs[0], TiebreakerBySonnebornBerger
	case model.TournamentSwiss:
		swissScoreTables := makeSwissTable(allMatches)

		// standard: player with the highest swiss score will win the tournament
		swissScoresWinnerIDs := findScoringTableWinners(swissScoreTables, func(int64) bool { return false })
		if len(swissScoresWinnerIDs) == 1 {
			return swissScoresWinnerIDs[0], TiebreakerNone
		}

		sonnebornTable := makeSonnebornTable(allMatches)

		// tiebreaker: use the Sonneborn-Berger score for each tied winner, largest wins
		sonnebornWinnerIDs := findScoringTableWinners(sonnebornTable, func(userID int64) bool { return !slices.Contains(swissScoresWinnerIDs, userID) })

		return sonnebornWinnerIDs[0], TiebreakerBySonnebornBerger
	default:
		panic(fmt.Sprintf("unknown tournament ruleset %s", ruleset))
	}
}

func makeWinCountTable(allMatches []CompletedPrevMatch) map[int64]int32 {
	winCountTable := make(map[int64]int32)
	for _, match := range allMatches {
		switch match.Result {
		case model.WhiteWin:
			winCountTable[match.WhiteID] = winCountTable[match.WhiteID] + 1
		case model.BlackWin:
			winCountTable[match.BlackID] = winCountTable[match.BlackID] + 1
		default:
		}
	}
	return winCountTable
}

func makeSonnebornTable(allMatches []CompletedPrevMatch) map[int64]float64 {
	sonnebornTable := make(map[int64]float64)
	for _, match := range allMatches {
		switch match.Result {
		case model.WhiteWin:
			sonnebornTable[match.WhiteID] = sonnebornTable[match.WhiteID] + match.BlackElo
		case model.BlackWin:
			sonnebornTable[match.BlackID] = sonnebornTable[match.BlackID] + match.WhiteElo
		case model.Draw:
			sonnebornTable[match.WhiteID] = sonnebornTable[match.WhiteID] + match.BlackElo/2
			sonnebornTable[match.BlackID] = sonnebornTable[match.BlackID] + match.WhiteElo/2
		}
	}
	return sonnebornTable
}

func makeSwissTable(allMatches []CompletedPrevMatch) map[int64]float64 {
	swissScores := make(map[int64]float64)

	for _, match := range allMatches {
		swissScores[match.WhiteID] = 0.0
		swissScores[match.BlackID] = 0.0
	}

	for _, match := range allMatches {
		switch match.Result {
		case model.WhiteWin:
			swissScores[match.WhiteID] = swissScores[match.WhiteID] + 1
		case model.BlackWin:
			swissScores[match.BlackID] = swissScores[match.BlackID] + 1
		case model.Draw:
			swissScores[match.WhiteID] = swissScores[match.WhiteID] + 0.5
			swissScores[match.BlackID] = swissScores[match.BlackID] + 0.5
		}
	}
	return swissScores
}

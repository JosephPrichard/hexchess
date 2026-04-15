package svc

import (
	"cmp"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"math"
	"slices"
)

// participantsAtRound returns the number of elements at a given depth.
// Each node holds 2 elements. At depth d, there are 2^(N-d) nodes.
func participantsAtRound(maxDepth, depth int) int { return 1 << (maxDepth - depth + 1) }

// NodesAtDepth returns the number of nodes at a given depth.
// Root (depth N) has 1 node; each level down doubles the count.
func matchesAtRound(maxDepth, depth int) int { return 1 << (maxDepth - depth) }

type TournamentMatchCreation struct {
	GameID   string
	GameMode GameMode
	WhiteID  int64
	BlackID  int64
}

type FirstMatchParticipant struct {
	UserID int64
	Elo    pgtype.Float8
}

type FirstMatchmakingRequest struct {
	Ruleset      TournamentRuleset
	Mode         GameMode
	Participants []FirstMatchParticipant
	TotalRounds  int32
	MakeGameID   func() string
}

type FirstMatchmakingResult struct {
	Matches     []TournamentMatchCreation
	TotalRounds int32 // RoundRobin and Swiss calculate total rounds during matchmaking rather than using already existing rounds to validate
}

func calcRoundRobinTournamentRounds(participantCount int) int32 {
	return int32((participantCount * (participantCount - 1)) / 2)
}

func calcSwissTournamentRounds(participantCount int) int32 {
	return int32(math.Log2(float64(participantCount)))
}

type StartTournamentError struct {
	Ruleset TournamentRuleset
	Err     error
}

func wrapStartTournamentError(ruleset TournamentRuleset, err error) StartTournamentError {
	return StartTournamentError{Ruleset: ruleset, Err: err}
}

func (e StartTournamentError) Error() string {
	return fmt.Sprintf("tournament ruleset %s with violation: %v", e.Ruleset, e.Err)
}

var (
	ErrInvalidParticipantCount  = fmt.Errorf("tournament does not have enough participant to create match")
	ErrInvalidParticipantParity = fmt.Errorf("participant count must be even")
	ErrMatchRoundCount          = errors.New("tournament has an invalid completed match count in round")
)

func makeMatchesLinearly(participants []FirstMatchParticipant, gameMode GameMode, makeGameID func() string) []TournamentMatchCreation {
	// invariant: participant count is always even (`elementsAtFirstDepth` always returns even)
	if len(participants)%2 != 0 {
		// assert rather than return an error because this property is statically encoded into the `ElementsAtFirstDepth` algorithm
		panic(fmt.Sprintf("participant count %+v is not even", participants))
	}
	var matches []TournamentMatchCreation
	for i := 0; i+1 < len(participants); i += 2 {
		matches = append(matches, TournamentMatchCreation{
			GameID:   makeGameID(),
			GameMode: gameMode,
			WhiteID:  participants[i].UserID,
			BlackID:  participants[i+1].UserID,
		})
	}
	return matches
}

func makeMatchesCrissCross(participants []FirstMatchParticipant, gameMode GameMode, makeGameID func() string) []TournamentMatchCreation {
	var matches []TournamentMatchCreation
	low := 0
	high := len(participants) - 1
	for low < high {
		matches = append(matches, TournamentMatchCreation{
			GameID:   makeGameID(),
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

func MakeFirstMatches(request FirstMatchmakingRequest) (result FirstMatchmakingResult, err error) {
	participantCount := len(request.Participants)

	var matches []TournamentMatchCreation
	totalRounds := request.TotalRounds

	switch request.Ruleset {
	case TournamentKnockout:
		// validation: matches are devided by two each time and stop at 1, we need to start at the expected power of 2
		if participantCount != participantsAtRound(int(request.TotalRounds), 1) {
			return result, wrapStartTournamentError(TournamentKnockout, ErrInvalidParticipantCount)
		}
		matches = makeMatchesLinearly(request.Participants, request.Mode, request.MakeGameID)
		// invariant: we should have as many matches expected at the first round
		if len(matches) != matchesAtRound(int(request.TotalRounds), 1) {
			return result, ErrMatchRoundCount
		}
	case TournamentRoundRobin:
		// validation: as long as we can match each player with another player, we can start the tournament
		if participantCount%2 != 0 {
			return result, wrapStartTournamentError(TournamentRoundRobin, ErrInvalidParticipantParity)
		}
		matches = makeMatchesLinearly(request.Participants, request.Mode, request.MakeGameID)

		totalRounds = calcRoundRobinTournamentRounds(participantCount)
	case TournamentSwiss:
		// validation: as long as we can match each player with another player, we can start the tournament
		if participantCount%2 != 0 {
			return result, wrapStartTournamentError(TournamentSwiss, ErrInvalidParticipantParity)
		}

		// a swiss tournament matches the worst players with the best players
		slices.SortFunc(request.Participants, func(a, b FirstMatchParticipant) int {
			if n := cmp.Compare(defaultElo(b.Elo), defaultElo(a.Elo)); n != 0 {
				return n
			}
			return cmp.Compare(a.UserID, b.UserID) // deterministic sort order
		})
		matches = makeMatchesCrissCross(request.Participants, request.Mode, request.MakeGameID)

		totalRounds = calcSwissTournamentRounds(participantCount)
	default:
		return result, fmt.Errorf("unknown tournament ruleset %s", request.Ruleset)
	}

	return FirstMatchmakingResult{Matches: matches, TotalRounds: totalRounds}, nil
}

type TieBreakerKind int

const (
	TiebreakerNone TieBreakerKind = iota
	TiebreakerByElo
	TiebreakerBySonnebornBerger
)

type PrevMatch struct {
	Round    int32
	WhiteID  int64
	BlackID  int64
	WhiteElo float64
	BlackElo float64
	Result   ReplayResult
}

func getKnockoutWinnerID(match PrevMatch) (int64, TieBreakerKind) {
	switch match.Result {
	case WhiteWin:
		return match.WhiteID, TiebreakerNone
	case BlackWin:
		return match.BlackID, TiebreakerNone
	// tiebreaker for draw uses the player with the highest elo, with white as a last case scenario
	case Draw:
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

type MatchmakingRequest struct {
	Ruleset     TournamentRuleset
	Matches     []PrevMatch
	GameMode    GameMode
	TotalRounds int32
	MakeGameID  func() string
}

type MatchmakingResult struct {
	NextMatches    []TournamentMatchCreation
	NextStatus     TournamentStatus
	NextMatchRound int32
	WinnerID       int64
	Tiebreaker     TieBreakerKind
}

func getPrevRoundMatches(matches []PrevMatch) []PrevMatch {
	prevMatchRound := matches[len(matches)-1].Round
	var prevRoundMatches []PrevMatch
	for _, match := range matches {
		if match.Round == prevMatchRound {
			prevRoundMatches = append(prevRoundMatches, match)
		}
	}
	return prevRoundMatches
}

func DoMatchmaking(request MatchmakingRequest) (MatchmakingResult, error) {
	// invariant: a tournament must have matches to do matchmaking
	if len(request.Matches) == 0 {
		return MatchmakingResult{}, ErrEmptyMatchesTournament
	}

	totalRounds := int(request.TotalRounds)
	gameMode := request.GameMode
	allMatches := request.Matches

	prevMatchRound := allMatches[len(allMatches)-1].Round

	var nextMatches []TournamentMatchCreation

	switch request.Ruleset {
	case TournamentKnockout:
		prevRoundMatches := getPrevRoundMatches(allMatches)

		// validation: `Knockout` the last batch of NextMatches are at a completed state
		if len(prevRoundMatches) == matchesAtRound(totalRounds, int(prevMatchRound)) {
			// additionally, this check makes this operation idempotent within a very short timeframe
			// if two matchmaking operations run serially, the second one will produce this error
			return MatchmakingResult{}, ErrMatchRoundCount
		}

		if len(prevRoundMatches)%2 != 0 {
			panic(fmt.Sprintf("match count %d is not even", len(prevRoundMatches)))
		}

		var nextMatches []TournamentMatchCreation
		for i := 0; i+1 < len(prevRoundMatches); i += 2 {
			matchOne := prevRoundMatches[i]
			matchTwo := prevRoundMatches[i+1]
			nextMatches = append(nextMatches, TournamentMatchCreation{
				GameID:   request.MakeGameID(),
				GameMode: gameMode,
				WhiteID:  withoutTiebreaker(getKnockoutWinnerID(matchOne)),
				BlackID:  withoutTiebreaker(getKnockoutWinnerID(matchTwo)),
			})
		}

		if len(nextMatches) == len(prevRoundMatches)/2 {
			panic(fmt.Sprintf("expected %d next matches, got %d", len(prevRoundMatches)/2, len(nextMatches)))
		}
	case TournamentRoundRobin:
		prevRoundMatches := getPrevRoundMatches(allMatches)

		var nextMatches []TournamentMatchCreation

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

			nextMatches = append(nextMatches, TournamentMatchCreation{
				GameID:   request.MakeGameID(),
				GameMode: gameMode,
				WhiteID:  nextWhiteID,
				BlackID:  nextBlackID,
			})
		}
	case TournamentSwiss:
		swissScoresTable := makeSwissTable(allMatches)

		// collect and reverse sort match participants by swiss score
		var participantIDs []int64
		for userID, _ := range swissScoresTable {
			participantIDs = append(participantIDs, userID)
		}
		slices.SortFunc(participantIDs, func(a, b int64) int {
			return cmp.Compare(swissScoresTable[b], swissScoresTable[a])
		})

		var matches []TournamentMatchCreation
		for i := 0; i+1 < len(participantIDs); i += 2 {
			matches = append(matches, TournamentMatchCreation{
				GameID:   request.MakeGameID(),
				GameMode: gameMode,
				WhiteID:  participantIDs[i],
				BlackID:  participantIDs[i+1],
			})
		}
	default:
		return MatchmakingResult{}, fmt.Errorf("unknown tournament ruleset %s", request.Ruleset)
	}

	nextMatchRound := prevMatchRound + 1

	nextStatus := TournamentInProgress
	winnerID := int64(0) // defaults to no winner
	tiebreaker := TiebreakerNone

	if nextMatchRound == request.TotalRounds {
		nextStatus = TournamentFinished
		winnerID, tiebreaker = findTournamentWinner(request.Ruleset, request.Matches)
	}

	return MatchmakingResult{NextMatches: nextMatches, NextStatus: nextStatus, NextMatchRound: nextMatchRound, WinnerID: winnerID, Tiebreaker: tiebreaker}, nil
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

func findTournamentWinner(ruleset TournamentRuleset, allMatches []PrevMatch) (int64, TieBreakerKind) {
	switch ruleset {
	case TournamentKnockout:
		// winner of the tournament is the player left standing
		return getKnockoutWinnerID(allMatches[len(allMatches)-1])
	case TournamentRoundRobin:
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
	case TournamentSwiss:
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

func makeWinCountTable(allMatches []PrevMatch) map[int64]int32 {
	winCountTable := make(map[int64]int32)
	for _, match := range allMatches {
		switch match.Result {
		case WhiteWin:
			winCountTable[match.WhiteID] = winCountTable[match.WhiteID] + 1
		case BlackWin:
			winCountTable[match.BlackID] = winCountTable[match.BlackID] + 1
		default:
		}
	}
	return winCountTable
}

func makeSonnebornTable(allMatches []PrevMatch) map[int64]float64 {
	sonnebornTable := make(map[int64]float64)
	for _, match := range allMatches {
		switch match.Result {
		case WhiteWin:
			sonnebornTable[match.WhiteID] = sonnebornTable[match.WhiteID] + match.BlackElo
		case BlackWin:
			sonnebornTable[match.BlackID] = sonnebornTable[match.BlackID] + match.WhiteElo
		case Draw:
			sonnebornTable[match.WhiteID] = sonnebornTable[match.WhiteID] + match.BlackElo/2
			sonnebornTable[match.BlackID] = sonnebornTable[match.BlackID] + match.WhiteElo/2
		}
	}
	return sonnebornTable
}

func makeSwissTable(allMatches []PrevMatch) map[int64]float64 {
	swissScores := make(map[int64]float64)
	for _, match := range allMatches {
		switch match.Result {
		case WhiteWin:
			swissScores[match.WhiteID] = swissScores[match.WhiteID] + 1
		case BlackWin:
			swissScores[match.BlackID] = swissScores[match.BlackID] + 1
		case Draw:
			swissScores[match.WhiteID] = swissScores[match.WhiteID] + 0.5
			swissScores[match.BlackID] = swissScores[match.BlackID] + 0.5
		}
	}
	return swissScores
}

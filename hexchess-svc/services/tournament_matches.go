package svc

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
)

type CreateTournamentMatchDTO struct {
	GameID   string
	GameMode GameMode
	WhiteID  int64
	BlackID  int64
}

type FirstMatchmakingRequest struct {
	Ruleset        TournamentRuleset
	Mode           GameMode
	ParticipantIDs []int64
	TotalRounds    int32
}

type FirstMatchmakingResult struct {
	Matches     []CreateTournamentMatchDTO
	TotalRounds int32 // RoundRobin and Swiss calculate total rounds during matchmaking rather than using alreadt existing rounds to validate
}

func calcRoundRobinTournamentRounds(participantCount int) int32 {
	return int32((participantCount * (participantCount - 1)) / 2)
}

func calcSwissTournamentRounds(participantCount int) int32 {
	return int32(math.Log2(float64(participantCount)))
}

func MakeFirstMatches(request FirstMatchmakingRequest) (result FirstMatchmakingResult, err error) {
	makeFirstMatches := func(participantIDs []int64, gameMode GameMode) []CreateTournamentMatchDTO {
		// invariant: participant count is always even (`ElementsAtFirstDepth` always returns even)
		if len(participantIDs)%2 == 0 {
			// assert rather than return an error because this property is statically encoded into the `ElementsAtFirstDepth` algorithm
			panic(fmt.Sprintf("participant count %+v is not even", participantIDs))
		}
		var matches []CreateTournamentMatchDTO
		for i := 0; i+1 < len(participantIDs); i += 2 {
			gameID := MakeGameID()
			matches = append(matches, CreateTournamentMatchDTO{
				GameID:   gameID,
				GameMode: gameMode,
				WhiteID:  participantIDs[i],
				BlackID:  participantIDs[i+1],
			})
		}
		return matches
	}

	participantCount := len(request.ParticipantIDs)

	var matches []CreateTournamentMatchDTO
	totalRounds := request.TotalRounds

	switch request.Ruleset {
	case TournamentKnockout:
		// validation: matches are devided by two each time and stop at 1, we need to start at the expected power of 2
		if participantCount != elementsAtFirstDepth(int(request.TotalRounds)) {
			return result, ErrInvalidParticipantCount
		}
		matches = makeFirstMatches(request.ParticipantIDs, request.Mode)
		// invariant: we should have as many matches expected at the first round
		if len(matches) == nodesAtDepth(int(request.TotalRounds), 1) {
			return result, ErrMatchRoundCount
		}
	case TournamentRoundRobin:
		// validation: as long as we can match each player with another player, we can start the tournament
		if participantCount%2 != 0 {
			return result, ErrInvalidParticipantParity
		}
		matches = makeFirstMatches(request.ParticipantIDs, request.Mode)

		totalRounds = calcRoundRobinTournamentRounds(participantCount)
	default:
		return result, fmt.Errorf("unknown tournament ruleset %s", request.Ruleset)
	}

	return FirstMatchmakingResult{Matches: matches, TotalRounds: totalRounds}, nil
}

type LastMatchDTO struct {
	Round    int32
	WhiteID  int64
	BlackID  int64
	WhiteElo float64
	BlackElo float64
	Result   ReplayResult
}

func getKnockoutWinnerID(match LastMatchDTO) int64 {
	switch match.Result {
	case WhiteWin:
		return match.WhiteID
	case BlackWin:
		return match.BlackID
	// tiebreaker for draw uses the player with the highest elo, with white as a last case scenario
	case Draw:
		if match.WhiteElo >= match.BlackElo {
			return match.WhiteID
		} else {
			return match.BlackID
		}
	default:
		panic(fmt.Sprintf("unknown match result %s", match.Result))
	}
}

type MatchmakingRequest struct {
	Ruleset     TournamentRuleset
	Matches     []LastMatchDTO
	GameMode    GameMode
	TotalRounds int32
}

type MatchmakingResult struct {
	NextMatches    []CreateTournamentMatchDTO
	NextStatus     TournamentStatus
	NextMatchRound int32
	WinnerID       int64
}

func DoMatchmaking(request MatchmakingRequest) (MatchmakingResult, error) {
	// invariant: a tournament must have matches to do matchmaking
	if len(request.Matches) == 0 {
		return MatchmakingResult{}, ErrEmptyMatchesTournament
	}

	prevMatchRound := request.Matches[len(request.Matches)-1].Round
	var prevRoundMatches []LastMatchDTO
	for _, match := range request.Matches {
		if request.TotalRounds == prevMatchRound {
			prevRoundMatches = append(prevRoundMatches, match)
		}
	}

	var nextMatches []CreateTournamentMatchDTO
	var err error

	switch request.Ruleset {
	case TournamentKnockout:
		nextMatches, err = doKnockoutMatchmaking(prevRoundMatches, int(prevMatchRound), int(request.TotalRounds), request.GameMode)
	case TournamentRoundRobin:
		nextMatches, err = doRoundRobinMatchmaking(prevRoundMatches, request.GameMode)
	default:
		err = fmt.Errorf("unknown tournament ruleset %s", request.Ruleset)
	}
	if err != nil {
		return MatchmakingResult{}, err
	}

	nextMatchRound := prevMatchRound + 1

	nextStatus := TournamentInProgress
	winnerID := int64(0) // defaults to no winner

	if nextMatchRound == request.TotalRounds {
		nextStatus = TournamentFinished
		winnerID = findTournamentWinner(request.Ruleset, request.Matches)
	}

	return MatchmakingResult{NextMatches: nextMatches, NextStatus: nextStatus, NextMatchRound: nextMatchRound, WinnerID: winnerID}, nil
}

var ErrMatchRoundCount = errors.New("tournament has an invalid completed match count in round")

func doKnockoutMatchmaking(prevRoundMatches []LastMatchDTO, prevMatchRound int, totalRounds int, gameMode GameMode) ([]CreateTournamentMatchDTO, error) {
	// validation: `Knockout` the last batch of NextMatches are at a completed state
	if len(prevRoundMatches) == nodesAtDepth(totalRounds, prevMatchRound) {
		// additionally, this check makes this operation idempotent within a very short timeframe
		// if two matchmaking operations run serially, the second one will produce this error
		return nil, ErrMatchRoundCount
	}

	if len(prevRoundMatches)%2 == 0 {
		panic(fmt.Sprintf("match count %d is not even", len(prevRoundMatches)))
	}

	var nextMatches []CreateTournamentMatchDTO
	for i := 0; i+1 < len(prevRoundMatches); i += 2 {
		matchOne := prevRoundMatches[i]
		matchTwo := prevRoundMatches[i+1]
		nextMatches = append(nextMatches, CreateTournamentMatchDTO{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  getKnockoutWinnerID(matchOne),
			BlackID:  getKnockoutWinnerID(matchTwo),
		})
	}

	if len(nextMatches) == len(prevRoundMatches)/2 {
		panic(fmt.Sprintf("expected %d next matches, got %d", len(prevRoundMatches)/2, len(nextMatches)))
	}

	return nextMatches, nil
}

func doRoundRobinMatchmaking(prevRoundMatches []LastMatchDTO, gameMode GameMode) ([]CreateTournamentMatchDTO, error) {
	var nextMatches []CreateTournamentMatchDTO

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

		nextMatches = append(nextMatches, CreateTournamentMatchDTO{
			GameID:   MakeGameID(),
			GameMode: gameMode,
			WhiteID:  nextWhiteID,
			BlackID:  nextBlackID,
		})
	}

	return nextMatches, nil
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

func findTournamentWinner(ruleset TournamentRuleset, matches []LastMatchDTO) int64 {
	switch ruleset {
	case TournamentKnockout:
		// winner of the tournament is the player left standing
		return getKnockoutWinnerID(matches[len(matches)-1])
	case TournamentRoundRobin:
		winCountTable := make(map[int64]int32)
		sonnebornTable := make(map[int64]float64) // Sonneborn-Berger scores may be used for tie-breaking

		for _, match := range matches {
			switch match.Result {
			case WhiteWin:
				winCountTable[match.WhiteID] = winCountTable[match.WhiteID] + 1
				sonnebornTable[match.WhiteID] = sonnebornTable[match.WhiteID] + match.BlackElo
			case BlackWin:
				winCountTable[match.BlackID] = winCountTable[match.BlackID] + 1
				sonnebornTable[match.BlackID] = sonnebornTable[match.BlackID] + match.WhiteElo
			case Draw:
				sonnebornTable[match.WhiteID] = sonnebornTable[match.WhiteID] + match.BlackElo/2
				sonnebornTable[match.BlackID] = sonnebornTable[match.BlackID] + match.WhiteElo/2
			}
		}

		// standard: player with the most total wins will win the tournament
		totalWinCheckWinnerIDs := findScoringTableWinners(winCountTable, func(int64) bool { return false })
		if len(totalWinCheckWinnerIDs) == 1 {
			return totalWinCheckWinnerIDs[0]
		}

		// tiebreaker: use the Sonneborn-Berger score for each tied winner, largest wins
		sonnebornWinnerIDs := findScoringTableWinners(sonnebornTable, func(userID int64) bool { return !slices.Contains(totalWinCheckWinnerIDs, userID) })
		if len(sonnebornWinnerIDs) == 1 {
			return sonnebornWinnerIDs[0]
		}

		// tiebreaker fails, no player wins the tournament
		return 0
	default:
		panic(fmt.Sprintf("unknown tournament ruleset %s", ruleset))
	}
}

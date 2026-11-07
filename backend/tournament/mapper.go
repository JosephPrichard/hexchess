package tournament

import (
	"hexchess-svc/database"
	"hexchess-svc/database/query"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/replay"
	"hexchess-svc/user"
	"hexchess-svc/userdata"
	"hexchess-svc/utils/enum"
	"time"
)

func mapTournamentRow(tournament query.SelectTournamentByIDRow) Tournament {
	ruleset := enum.Expect(tournament.Ruleset, TournamentRulesetEnums)
	status := enum.Expect(tournament.Status, TournamentStatusEnums)
	mode := enum.Expect(tournament.Mode, gamedata.GameModeEnums)

	maxPlayerCount := maxPlayerCountTournament(ruleset, tournament.Rounds)
	countdown := time.Duration(tournament.Countdown) * time.Millisecond

	return Tournament{
		ID:                 tournament.ID,
		TournamentKey:      tournament.TournamentKey.Bytes,
		Name:               tournament.Name,
		Rounds:             tournament.Rounds,
		WinnerID:           tournament.WinnerID.Int64,
		MaxPlayerCount:     maxPlayerCount,
		CountdownStartedOn: tournament.CountdownStartedOn.Time,
		CountdownStarted:   tournament.CountdownStartedOn.Valid,
		Countdown:          countdown.String(),
		CreatedOn:          tournament.CreatedOn.Time,
		CreatedBy:          tournament.CreatedBy,
		Status:             status,
		Ruleset:            ruleset,
		Mode:               mode,
	}
}

func mapTournamentsByParticipantRows(tournamentRows []query.SelectTournamentsByParticipantRow) []Tournament {
	var tournaments []Tournament
	for _, row := range tournamentRows {
		tournaments = append(tournaments, mapTournamentRow(query.SelectTournamentByIDRow(row)))
	}
	return tournaments
}

func mapTournamentsRows(tournamentRows []query.SelectTournamentsRow) []Tournament {
	var tournaments []Tournament
	for _, row := range tournamentRows {
		tournaments = append(tournaments, mapTournamentRow(query.SelectTournamentByIDRow(row)))
	}
	return tournaments
}

func maxPlayerCountTournament(ruleset TournamentRuleset, rounds int32) int {
	if ruleset == TournamentKnockout {
		return KnockoutParticipantsAtRound(int(rounds), 1)
	}
	return -1
}

func mapTourneyParticipantFromRow(participant query.SelectParticipantsWithUserByTournamentIDRow) Participant {
	return user.LbdUser{
		ID:         participant.UserID,
		Username:   participant.Username,
		Country:    participant.Country,
		JoinedOn:   participant.UserJoinedOn.Time,
		Elo:        userdata.DefaultUserElo(participant.Elo),
		HighestElo: userdata.DefaultUserElo(participant.HighestElo),
		Wins:       participant.Wins.Int32,
		Losses:     participant.Losses.Int32,
		Winrate:    userdata.CalcUserWinrate(participant.Wins.Int32, participant.Losses.Int32, participant.Draws.Int32),
	}
}

func mapTourneyMatchFromRow(match query.SelectReplayMatchesByTournamentIDRow) FullMatch {
	var tournamentReplay *TournamentReplay

	// invariant: if replayID is non-null, all other replay columns will also be non null.
	if match.ReplayID.Valid {
		replayResult := enum.Expect(match.Result.ResultEnum, replay.ReplayResultEnums)
		replayCause := enum.Expect(match.Cause.CauseEnum, replay.ReplayCauseEnums)
		replayMode := enum.Expect(match.Mode.ModeEnum, gamedata.GameModeEnums)

		replayData := replay.Replay{
			ID:          match.ReplayID.Int64,
			WhiteID:     match.WhiteID,
			BlackID:     match.BlackID,
			Result:      replayResult,
			Cause:       replayCause,
			Mode:        replayMode,
			WinEloDiff:  match.WinEloDiff.Float64,
			LoseEloDiff: match.LoseEloDiff.Float64,
			PlayedOn:    match.PlayedOn.Time,
		}
		tournamentReplay = &TournamentReplay{
			Replay:          replayData,
			ReplayColorElos: replay.NewReplayView(replayData),
		}
	}

	return FullMatch{
		Ordering:      match.Ordering,
		GameID:        gameid.Identifier(match.GameID), // null gameID will be an empty string.
		TournamentKey: match.TournamentKey.Bytes,
		Round:         match.Round,
		CreatedOn:     match.CreatedOn.Time,
		Replay:        tournamentReplay,
		WhiteID:       match.WhiteID,
		BlackID:       match.BlackID,
	}
}

func mapFullTournament(
	tournamentRow query.SelectTournamentByIDRow,
	matchRows []query.SelectReplayMatchesByTournamentIDRow,
	participantRows []query.SelectParticipantsWithUserByTournamentIDRow,
) FullTournament {
	tournament := mapTournamentRow(tournamentRow)

	participants := make([]Participant, 0, len(participantRows))
	for _, row := range participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	matches := make([]FullMatch, 0, len(matchRows))
	for _, row := range matchRows {
		matches = append(matches, mapTourneyMatchFromRow(row))
	}

	return FullTournament{Tournament: tournament, Participants: participants, Matches: matches}
}

func mapParticipantInsertErr(err error) error {
	return database.MapInsertErr(err, ErrTournamentAlreadyJoined, ErrInvalidTournamentParticipant)
}

package tournament

import (
	"hexchess-svc/database"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/service/leaderboard"
	"hexchess-svc/utils/enum"
	"time"
)

func mapTournamentByIdRow(tournament query.SelectTournamentByIDRow) model.Tournament {
	ruleset := enum.Expect(tournament.Ruleset, model.TournamentRulesetEnums)
	status := enum.Expect(tournament.Status, model.TournamentStatusEnums)
	mode := enum.Expect(tournament.Mode, model.GameModeEnums)

	maxPlayerCount := maxPlayerCountTournament(ruleset, tournament.Rounds)
	countdown := time.Duration(tournament.Countdown) * time.Millisecond

	return model.Tournament{
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

func mapTourneyParticipantFromRow(participant query.SelectParticipantsWithUserByTournamentIDRow) model.Participant {
	return leaderboard.MapLeaderboardUser(query.SelectUserWithEloByIDRow{
		ID:         participant.UserID,
		Username:   participant.Username,
		Country:    participant.Country,
		Bio:        participant.Bio,
		JoinedOn:   participant.UserJoinedOn,
		Elo:        participant.Elo,
		HighestElo: participant.HighestElo,
		Wins:       participant.Wins,
		Losses:     participant.Losses,
		Draws:      participant.Draws,
	})
}

func mapTourneyMatchFromRow(match query.SelectReplayMatchesByTournamentIDRow) model.FullMatch {
	var tournamentReplay *model.TournamentReplay

	// invariant: if replayID is non-null, all other replay columns will also be non null.
	if match.ReplayID.Valid {
		replayResult := enum.Expect(match.Result.ResultEnum, model.ReplayResultEnums)
		replayCause := enum.Expect(match.Cause.CauseEnum, model.ReplayCauseEnums)
		replayMode := enum.Expect(match.Mode.ModeEnum, model.GameModeEnums)

		replay := model.Replay{
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
		tournamentReplay = &model.TournamentReplay{
			Replay:          replay,
			ReplayColorElos: model.NewReplayView(replay),
		}
	}

	return model.FullMatch{
		Ordering:      match.Ordering,
		GameID:        model.GameID(match.GameID), // null gameID will be an empty string.
		TournamentKey: match.TournamentKey.Bytes,
		Round:         match.Round,
		CreatedOn:     match.CreatedOn.Time,
		Replay:        tournamentReplay,
		WhiteID:       match.WhiteID,
		BlackID:       match.BlackID,
	}
}

func mapFullTournament(tournamentRow query.SelectTournamentByIDRow, matchRows []query.SelectReplayMatchesByTournamentIDRow, participantRows []query.SelectParticipantsWithUserByTournamentIDRow) model.FullTournament {
	tournament := mapTournamentByIdRow(tournamentRow)

	participants := make([]model.Participant, 0, len(participantRows))
	for _, row := range participantRows {
		participants = append(participants, mapTourneyParticipantFromRow(row))
	}

	matches := make([]model.FullMatch, 0, len(matchRows))
	for _, row := range matchRows {
		matches = append(matches, mapTourneyMatchFromRow(row))
	}

	return model.FullTournament{Tournament: tournament, Participants: participants, Matches: matches}
}

func mapTournamentRows[Row interface {
	query.SelectTournamentsRow | query.SelectTournamentsByParticipantRow
}](
	tournamentRows []Row,
	fn func(tournament Row) model.Tournament,
) []model.Tournament {
	var tournaments []model.Tournament
	for _, row := range tournamentRows {
		tournaments = append(tournaments, fn(row))
	}
	return tournaments
}

func mapParticipantInsertErr(err error) error {
	return database.MapInsertErr(err, ErrTournamentAlreadyJoined, ErrInvalidTournamentParticipant)
}

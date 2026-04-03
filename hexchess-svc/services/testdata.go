package svc

import (
	"hexchess-svc/itest"
	"time"
)

var TestUserDTOs = []UserDTO{
	{
		ID:       1,
		Username: "user1",
		Country:  "us",
		Bio:      "",
		JoinedOn: itest.TimeNow.Local(),
	},
	{
		ID:       2,
		Username: "user2",
		Country:  "us",
		Bio:      "",
		JoinedOn: itest.TimeNow.Local(),
	},
}

var TestUserStats = []UserStatsDTO{
	{
		TotalWins:    17,
		TotalLosses:  14,
		AvgElo:       1025,
		HighestElo:   1050,
		TotalWinrate: 53,
		ModeStats: []ModeStatsDTO{
			{Mode: ModeCorrespondence7, Rank: 1, Wins: 2, Losses: 2, Winrate: 50, Elo: 1000, HighestElo: 1000},
			{Mode: ModeTimed3Plus2, Rank: 1, Wins: 5, Losses: 4, Winrate: 55, Elo: 1020, HighestElo: 1020},
			{Mode: ModeTimed15Plus10, Rank: 1, Wins: 4, Losses: 3, Winrate: 57, Elo: 1030, HighestElo: 1030},
			{Mode: ModeTimed1Plus0, Rank: 1, Wins: 6, Losses: 5, Winrate: 54, Elo: 1050, HighestElo: 1050},
		},
	},
}

var TestReplayDTOs = []FullReplayDTO{
	{
		ReplayDTO: ReplayDTO{
			ID:          1,
			WhiteID:     1,
			BlackID:     2,
			Mode:        ModeCorrespondence7,
			Result:      WhiteWin,
			Cause:       Checkmate,
			WinEloDiff:  30,
			LoseEloDiff: -30,
			PlayedOn:    itest.TimeNow.Local(),
		},
		ReplayUsersDTO: ReplayUsersDTO{
			WhiteName:    "user1",
			BlackName:    "user2",
			WhiteCountry: "us",
			BlackCountry: "us",
			WhiteElo:     1000,
			BlackElo:     1000,
		},
		ReplayViewDTO: ReplayViewDTO{
			WhiteEloDiff: 30,
			BlackEloDiff: -30,
		},
	},
	{
		ReplayDTO: ReplayDTO{
			ID:          3,
			WhiteID:     3,
			BlackID:     1,
			Mode:        ModeCorrespondence7,
			Result:      Draw,
			Cause:       Checkmate,
			WinEloDiff:  0,
			LoseEloDiff: 0,
			PlayedOn:    itest.TimeNow.Local(),
		},
		ReplayUsersDTO: ReplayUsersDTO{
			WhiteName:    "user3",
			BlackName:    "user1",
			WhiteCountry: "us",
			BlackCountry: "us",
			WhiteElo:     900,
			BlackElo:     1000,
		},
		ReplayViewDTO: ReplayViewDTO{
			WhiteEloDiff: 0,
			BlackEloDiff: 0,
		},
	},
	{
		ReplayDTO: ReplayDTO{
			ID:          4,
			WhiteID:     1,
			BlackID:     0,
			Mode:        ModeCorrespondence7,
			Result:      WhiteWin,
			Cause:       Checkmate,
			WinEloDiff:  0,
			LoseEloDiff: 0,
			PlayedOn:    itest.TimeNow.Local(),
		},
		ReplayUsersDTO: ReplayUsersDTO{
			WhiteName:    "user1",
			BlackName:    "",
			WhiteCountry: "us",
			BlackCountry: "",
			WhiteElo:     1000,
			BlackElo:     1000,
		},
		ReplayViewDTO: ReplayViewDTO{
			WhiteEloDiff: 0,
			BlackEloDiff: 0,
		},
	},
}

var TestChallengeDTOs = []ChallengeDTO{
	{
		ChallengerID:      1,
		ChallengerName:    "user1",
		ChallengerCountry: "us",
		ChallengerElo:     1020,
		ChallengeeID:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		Mode:              ModeTimed3Plus2,
		StartColor:        Random,
		MadeOn:            itest.TimeNow.Local(),
		ExpiresOn:         itest.TimeNow.Local().Add(ExpireChallengeMaxAge),
	},
	{
		ChallengerID:      3,
		ChallengerName:    "user3",
		ChallengerCountry: "us",
		ChallengerElo:     900,
		ChallengeeID:      1,
		ChallengeeName:    "user1",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		Mode:              ModeCorrespondence1,
		StartColor:        Random,
		MadeOn:            itest.TimeNow.Local(),
		ExpiresOn:         itest.TimeNow.Local().Add(ExpireChallengeMaxAge),
	},
	{
		ChallengerID:      5,
		ChallengerName:    "user5",
		ChallengerCountry: "us",
		ChallengerElo:     1500,
		ChallengeeID:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		Mode:              ModeCorrespondence1,
		StartColor:        Random,
		MadeOn:            itest.TimeNow.Local(),
		ExpiresOn:         itest.TimeNow.Local().Add(ExpireChallengeMaxAge),
	},
	{
		ChallengerID:      5,
		ChallengerName:    "user5",
		ChallengerCountry: "us",
		ChallengerElo:     1500,
		ChallengeeID:      4,
		ChallengeeName:    "user4",
		ChallengeeCountry: "us",
		ChallengeeElo:     2000,
		Mode:              ModeCorrespondence1,
		StartColor:        Random,
		MadeOn:            itest.TimeNow.Local(),
		ExpiresOn:         itest.TimeNow.Local().Add(ExpireChallengeMaxAge),
	},
}

var TournamentDTOs = []TournamentDTO{
	{
		ID:             1,
		TournamentKey:  itest.TournamentInsts[0].TournamentKey,
		Name:           "Test Tournament 1",
		Depth:          2,
		MaxPlayerCount: 4,
		ScheduledOn:    time.Time{},
		IsScheduled:    false,
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Status:         TournamentLobby,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             2,
		TournamentKey:  itest.TournamentInsts[1].TournamentKey,
		Name:           "Test Tournament 2",
		Depth:          2,
		MaxPlayerCount: 4,
		ScheduledOn:    time.Time{},
		IsScheduled:    false,
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Status:         TournamentInProgress,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             3,
		TournamentKey:  itest.TournamentInsts[2].TournamentKey,
		Name:           "Test Tournament 3",
		Depth:          1,
		MaxPlayerCount: 2,
		ScheduledOn:    time.Time{},
		IsScheduled:    false,
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Status:         TournamentFinished,
		Mode:           ModeCorrespondence1,
	},
}

var MatchDTOs = []MatchDTO{
	{
		ID:            1,
		GameID:        *itest.TournamentMatchInsts[0].GameID,
		CreatedOn:     itest.TimeNow.Add(time.Minute * 1),
		TournamentKey: itest.TournamentInsts[1].TournamentKey,
		Depth:         1,
	},
	{
		ID:            2,
		GameID:        *itest.TournamentMatchInsts[1].GameID,
		CreatedOn:     itest.TimeNow.Add(time.Minute * 2),
		TournamentKey: itest.TournamentInsts[1].TournamentKey,
		Depth:         1,
	},
}

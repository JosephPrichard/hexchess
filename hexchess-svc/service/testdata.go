package svc

import (
	"hexchess-svc/itest"
	"time"
)

var TestUser = []User{
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

var TestUserStats = []UserStats{
	{
		TotalWins:    17,
		TotalLosses:  14,
		AvgElo:       1025,
		HighestElo:   1050,
		TotalWinrate: 53,
		ModeStats: []ModeStats{
			{Mode: ModeCorrespondence7, Rank: 1, Wins: 2, Losses: 2, Winrate: 50, Elo: 1000, HighestElo: 1000},
			{Mode: ModeTimed3Plus2, Rank: 1, Wins: 5, Losses: 4, Winrate: 55, Elo: 1020, HighestElo: 1020},
			{Mode: ModeTimed15Plus10, Rank: 1, Wins: 4, Losses: 3, Winrate: 57, Elo: 1030, HighestElo: 1030},
			{Mode: ModeTimed1Plus0, Rank: 1, Wins: 6, Losses: 5, Winrate: 54, Elo: 1050, HighestElo: 1050},
		},
	},
}

var TestReplay = []FullReplay{
	{
		Replay: Replay{
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
		ReplayUsers: ReplayUsers{
			WhiteName:    "user1",
			BlackName:    "user2",
			WhiteCountry: "us",
			BlackCountry: "us",
			WhiteElo:     1000,
			BlackElo:     1000,
		},
		RepayView: RepayView{
			WhiteEloDiff: 30,
			BlackEloDiff: -30,
		},
	},
	{
		Replay: Replay{
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
		ReplayUsers: ReplayUsers{
			WhiteName:    "user3",
			BlackName:    "user1",
			WhiteCountry: "us",
			BlackCountry: "us",
			WhiteElo:     900,
			BlackElo:     1000,
		},
		RepayView: RepayView{
			WhiteEloDiff: 0,
			BlackEloDiff: 0,
		},
	},
	{
		Replay: Replay{
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
		ReplayUsers: ReplayUsers{
			WhiteName:    "user1",
			BlackName:    "",
			WhiteCountry: "us",
			BlackCountry: "",
			WhiteElo:     1000,
			BlackElo:     1000,
		},
		RepayView: RepayView{
			WhiteEloDiff: 0,
			BlackEloDiff: 0,
		},
	},
}

var TestChallenge = []Challenge{
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

var Tournaments = []Tournament{
	{
		ID:             1,
		TournamentKey:  itest.Tournament0LobbyKey,
		Name:           "Test Tournament 0",
		Rounds:         2,
		MaxPlayerCount: 4,
		Countdown:      "5m1s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Status:         TournamentLobby,
		Ruleset:        TournamentKnockout,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             2,
		TournamentKey:  itest.Tournament1LobbyFilledKey,
		Name:           "Test Tournament 1",
		Rounds:         1,
		MaxPlayerCount: 2,
		Countdown:      "5m2s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Mode:           ModeCorrespondence7,
	},
	{
		ID:             3,
		TournamentKey:  itest.Tournament2ScheduledKnockoutKey,
		Name:           "Test Tournament 2",
		Rounds:         2,
		MaxPlayerCount: 4,
		Countdown:      "5m3s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentKnockout,
		Status:         TournamentScheduled,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             4,
		TournamentKey:  itest.Tournament3ScheduledRoundRobinKey,
		Name:           "Test Tournament 3",
		Rounds:         0,
		MaxPlayerCount: -1,
		Countdown:      "10m0s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentRoundRobin,
		Status:         TournamentScheduled,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             5,
		TournamentKey:  itest.Tournament4ScheduledSwissKey,
		Name:           "Test Tournament 4",
		Rounds:         0,
		MaxPlayerCount: -1,
		Countdown:      "11m0s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentSwiss,
		Status:         TournamentScheduled,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             6,
		TournamentKey:  itest.Tournament5InProgressKnockoutKey,
		Name:           "Test Tournament 5",
		Rounds:         2,
		MaxPlayerCount: 4,
		Countdown:      "12m0s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentKnockout,
		Status:         TournamentInProgress,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             7,
		TournamentKey:  itest.Tournament6InProgressRoundRobinKey,
		Name:           "Test Tournament 6",
		Rounds:         0,
		MaxPlayerCount: -1,
		Countdown:      "1h1m1s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentRoundRobin,
		Status:         TournamentInProgress,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             8,
		TournamentKey:  itest.Tournament7InProgressSwissKey,
		Name:           "Test Tournament 7",
		Rounds:         0,
		MaxPlayerCount: -1,
		Countdown:      "1h2m2.002s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentSwiss,
		Status:         TournamentInProgress,
		Mode:           ModeCorrespondence1,
	},
	{
		ID:             9,
		TournamentKey:  itest.Tournament8FinishedKey,
		Name:           "Test Tournament 8",
		Rounds:         1,
		MaxPlayerCount: -1,
		Countdown:      "5m0s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Status:         TournamentFinished,
		Ruleset:        TournamentSwiss,
		Mode:           ModeCorrespondence1,
	},
}

var MatchTournament5 = []Match{
	{
		Ordering:      2,
		GameID:        itest.TournamentMatchInsts[1].GameID,
		CreatedOn:     itest.TimeNow.Add(time.Minute * 2),
		TournamentKey: itest.Tournament5InProgressKnockoutKey,
		Round:         1,
		WhiteID:       3,
		BlackID:       4,
	},
	{
		Ordering:      1,
		GameID:        itest.TournamentMatchInsts[0].GameID,
		CreatedOn:     itest.TimeNow.Add(time.Minute * 1),
		TournamentKey: itest.Tournament5InProgressKnockoutKey,
		Round:         1,
		WhiteID:       1,
		BlackID:       2,
	},
}

var MatchTournament8 = []Match{
	{
		Ordering:      3,
		GameID:        itest.TournamentMatchInsts[2].GameID,
		CreatedOn:     itest.TimeNow.Add(time.Minute * 3),
		TournamentKey: itest.Tournament8FinishedKey,
		Round:         1,
		WhiteID:       1,
		BlackID:       2,
		Replay: &TournamentReplay{
			Replay: Replay{
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
			RepayView: RepayView{WhiteEloDiff: 30, BlackEloDiff: -30},
		},
	},
}

var TournamentLbdChangeSets = []UpdtLbChangeSet{
	{ModeCorrespondence1, 4, 1400},
	{ModeCorrespondence1, 3, 1300},
	{ModeCorrespondence1, 2, 1200},
	{ModeCorrespondence1, 1, 1100},
}

// Tournament5RankedParticipants Ordered by `JoinedOn`, ranked with values in `TournamentLbdChangeSets`
var Tournament5RankedParticipants = []Participant{
	{
		User:       User{ID: 4, Username: "user4", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        2000,
		HighestElo: 2000,
		Rank:       1,
	},
	{
		User:       User{ID: 3, Username: "user3", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        900,
		HighestElo: 900,
		Rank:       2,
	},
	{
		User:       User{ID: 2, Username: "user2", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       3,
	},
	{
		User:       User{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       4,
	},
}

// Tournament8RankedParticipants Ordered by `JoinedOn`, ranked with values in `TournamentLbdChangeSets`
var Tournament8RankedParticipants = []Participant{
	{
		User:       User{ID: 2, Username: "user2", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       3,
	},
	{
		User:       User{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       4,
	},
}

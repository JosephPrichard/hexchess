package itest

import (
	"hexchess-svc/domain"
	"time"
)

var TestUser = []domain.User{
	{
		ID:       1,
		Username: "user1",
		Country:  "us",
		Bio:      "",
		JoinedOn: TimeNow.Local(),
	},
	{
		ID:       2,
		Username: "user2",
		Country:  "us",
		Bio:      "",
		JoinedOn: TimeNow.Local(),
	},
}

var TestUserStats = []domain.UserStats{
	{
		TotalWins:    17,
		TotalLosses:  14,
		AvgElo:       1025,
		HighestElo:   1050,
		TotalWinrate: 53,
		ModeStats: []domain.ModeStats{
			{Mode: domain.ModeCorrespondence7, Rank: 1, Wins: 2, Losses: 2, Winrate: 50, Elo: 1000, HighestElo: 1000},
			{Mode: domain.ModeTimed3Plus2, Rank: 1, Wins: 5, Losses: 4, Winrate: 55, Elo: 1020, HighestElo: 1020},
			{Mode: domain.ModeTimed15Plus10, Rank: 1, Wins: 4, Losses: 3, Winrate: 57, Elo: 1030, HighestElo: 1030},
			{Mode: domain.ModeTimed1Plus0, Rank: 1, Wins: 6, Losses: 5, Winrate: 54, Elo: 1050, HighestElo: 1050},
		},
	},
}

var TestReplay = []domain.FullReplay{
	{
		Replay: domain.Replay{
			ID:          1,
			WhiteID:     1,
			BlackID:     2,
			Mode:        domain.ModeCorrespondence7,
			Result:      domain.WhiteWin,
			Cause:       domain.Checkmate,
			WinEloDiff:  30,
			LoseEloDiff: -30,
			PlayedOn:    TimeNow.Local(),
		},
		ReplayUsers: domain.ReplayUsers{
			WhiteName:    "user1",
			BlackName:    "user2",
			WhiteCountry: "us",
			BlackCountry: "us",
			WhiteElo:     1000,
			BlackElo:     1000,
		},
		RepayView: domain.RepayView{
			WhiteEloDiff: 30,
			BlackEloDiff: -30,
		},
	},
	{
		Replay: domain.Replay{
			ID:          3,
			WhiteID:     3,
			BlackID:     1,
			Mode:        domain.ModeCorrespondence7,
			Result:      domain.Draw,
			Cause:       domain.Checkmate,
			WinEloDiff:  0,
			LoseEloDiff: 0,
			PlayedOn:    TimeNow.Local(),
		},
		ReplayUsers: domain.ReplayUsers{
			WhiteName:    "user3",
			BlackName:    "user1",
			WhiteCountry: "us",
			BlackCountry: "us",
			WhiteElo:     900,
			BlackElo:     1000,
		},
		RepayView: domain.RepayView{
			WhiteEloDiff: 0,
			BlackEloDiff: 0,
		},
	},
	{
		Replay: domain.Replay{
			ID:          4,
			WhiteID:     1,
			BlackID:     0,
			Mode:        domain.ModeCorrespondence7,
			Result:      domain.WhiteWin,
			Cause:       domain.Checkmate,
			WinEloDiff:  0,
			LoseEloDiff: 0,
			PlayedOn:    TimeNow.Local(),
		},
		ReplayUsers: domain.ReplayUsers{
			WhiteName:    "user1",
			BlackName:    "",
			WhiteCountry: "us",
			BlackCountry: "",
			WhiteElo:     1000,
			BlackElo:     1000,
		},
		RepayView: domain.RepayView{
			WhiteEloDiff: 0,
			BlackEloDiff: 0,
		},
	},
}

var TestChallenge = []domain.Challenge{
	{
		ChallengerID:      1,
		ChallengerName:    "user1",
		ChallengerCountry: "us",
		ChallengerElo:     1020,
		ChallengeeID:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		Mode:              domain.ModeTimed3Plus2,
		StartColor:        domain.Random,
		MadeOn:            TimeNow.Local(),
		ExpiresOn:         TimeNow.Local().Add(time.Hour * 24 * 7),
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
		Mode:              domain.ModeCorrespondence1,
		StartColor:        domain.Random,
		MadeOn:            TimeNow.Local(),
		ExpiresOn:         TimeNow.Local().Add(time.Hour * 24 * 7),
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
		Mode:              domain.ModeCorrespondence1,
		StartColor:        domain.Random,
		MadeOn:            TimeNow.Local(),
		ExpiresOn:         TimeNow.Local().Add(time.Hour * 24 * 7),
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
		Mode:              domain.ModeCorrespondence1,
		StartColor:        domain.Random,
		MadeOn:            TimeNow.Local(),
		ExpiresOn:         TimeNow.Local().Add(time.Hour * 24 * 7),
	},
}

var Tournaments = []domain.Tournament{
	{
		ID:             1,
		TournamentKey:  Tournament0LobbyKey,
		Name:           "Test Tournament 0",
		Rounds:         2,
		MaxPlayerCount: 4,
		Countdown:      "5m1s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Status:         domain.TournamentLobby,
		Ruleset:        domain.TournamentKnockout,
		Mode:           domain.ModeCorrespondence1,
	},
	{
		ID:             2,
		TournamentKey:  Tournament1LobbyFilledKey,
		Name:           "Test Tournament 1",
		Rounds:         1,
		MaxPlayerCount: 2,
		Countdown:      "5m2s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Mode:           domain.ModeCorrespondence7,
	},
	{
		ID:             3,
		TournamentKey:  Tournament2ScheduledKnockoutKey,
		Name:           "Test Tournament 2",
		Rounds:         2,
		MaxPlayerCount: 4,
		Countdown:      "5m3s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Ruleset:        domain.TournamentKnockout,
		Status:         domain.TournamentScheduled,
		Mode:           domain.ModeCorrespondence1,
	},
	{
		ID:             4,
		TournamentKey:  Tournament3ScheduledRoundRobinKey,
		Name:           "Test Tournament 3",
		Rounds:         0,
		MaxPlayerCount: -1,
		Countdown:      "10m0s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Ruleset:        domain.TournamentRoundRobin,
		Status:         domain.TournamentScheduled,
		Mode:           domain.ModeCorrespondence1,
	},
	{
		ID:             5,
		TournamentKey:  Tournament4ScheduledSwissKey,
		Name:           "Test Tournament 4",
		Rounds:         0,
		MaxPlayerCount: -1,
		Countdown:      "11m0s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Ruleset:        domain.TournamentSwiss,
		Status:         domain.TournamentScheduled,
		Mode:           domain.ModeCorrespondence1,
	},
	{
		ID:             6,
		TournamentKey:  Tournament5InProgressKnockoutKey,
		Name:           "Test Tournament 5",
		Rounds:         2,
		MaxPlayerCount: 4,
		Countdown:      "12m0s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Ruleset:        domain.TournamentKnockout,
		Status:         domain.TournamentInProgress,
		Mode:           domain.ModeCorrespondence1,
	},
	{
		ID:             7,
		TournamentKey:  Tournament6InProgressRoundRobinKey,
		Name:           "Test Tournament 6",
		Rounds:         0,
		MaxPlayerCount: -1,
		Countdown:      "1h1m1s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Ruleset:        domain.TournamentRoundRobin,
		Status:         domain.TournamentInProgress,
		Mode:           domain.ModeCorrespondence1,
	},
	{
		ID:             8,
		TournamentKey:  Tournament7InProgressSwissKey,
		Name:           "Test Tournament 7",
		Rounds:         0,
		MaxPlayerCount: -1,
		Countdown:      "1h2m2.002s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Ruleset:        domain.TournamentSwiss,
		Status:         domain.TournamentInProgress,
		Mode:           domain.ModeCorrespondence1,
	},
	{
		ID:             9,
		TournamentKey:  Tournament8FinishedKey,
		Name:           "Test Tournament 8",
		Rounds:         1,
		MaxPlayerCount: -1,
		Countdown:      "5m0s",
		CreatedOn:      TimeNow,
		CreatedBy:      1,
		Status:         domain.TournamentFinished,
		Ruleset:        domain.TournamentSwiss,
		Mode:           domain.ModeCorrespondence1,
	},
}

var MatchTournament5 = []domain.Match{
	{
		Ordering:      2,
		GameID:        TournamentMatchInsts[1].GameID,
		CreatedOn:     TimeNow.Add(time.Minute * 2),
		TournamentKey: Tournament5InProgressKnockoutKey,
		Round:         1,
		WhiteID:       3,
		BlackID:       4,
	},
	{
		Ordering:      1,
		GameID:        TournamentMatchInsts[0].GameID,
		CreatedOn:     TimeNow.Add(time.Minute * 1),
		TournamentKey: Tournament5InProgressKnockoutKey,
		Round:         1,
		WhiteID:       1,
		BlackID:       2,
	},
}

var MatchTournament8 = []domain.Match{
	{
		Ordering:      3,
		GameID:        TournamentMatchInsts[2].GameID,
		CreatedOn:     TimeNow.Add(time.Minute * 3),
		TournamentKey: Tournament8FinishedKey,
		Round:         1,
		WhiteID:       1,
		BlackID:       2,
		Replay: &domain.TournamentReplay{
			Replay: domain.Replay{
				ID:          1,
				WhiteID:     1,
				BlackID:     2,
				Mode:        domain.ModeCorrespondence7,
				Result:      domain.WhiteWin,
				Cause:       domain.Checkmate,
				WinEloDiff:  30,
				LoseEloDiff: -30,
				PlayedOn:    TimeNow.Local(),
			},
			RepayView: domain.RepayView{WhiteEloDiff: 30, BlackEloDiff: -30},
		},
	},
}

var TournamentLbdChangeSets = []struct {
	Mode    domain.GameMode
	ID      int64
	EloDiff float64
}{
	{domain.ModeCorrespondence1, 4, 1400},
	{domain.ModeCorrespondence1, 3, 1300},
	{domain.ModeCorrespondence1, 2, 1200},
	{domain.ModeCorrespondence1, 1, 1100},
}

// Tournament5RankedParticipants Ordered by `JoinedOn`, ranked with values in `TournamentLbdChangeSets`
var Tournament5RankedParticipants = []domain.Participant{
	{
		User:       domain.User{ID: 4, Username: "user4", Country: "us", JoinedOn: TimeNow},
		Elo:        2000,
		HighestElo: 2000,
		Rank:       1,
	},
	{
		User:       domain.User{ID: 3, Username: "user3", Country: "us", JoinedOn: TimeNow},
		Elo:        900,
		HighestElo: 900,
		Rank:       2,
	},
	{
		User:       domain.User{ID: 2, Username: "user2", Country: "us", JoinedOn: TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       3,
	},
	{
		User:       domain.User{ID: 1, Username: "user1", Country: "us", JoinedOn: TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       4,
	},
}

// Tournament8RankedParticipants Ordered by `JoinedOn`, ranked with values in `TournamentLbdChangeSets`
var Tournament8RankedParticipants = []domain.Participant{
	{
		User:       domain.User{ID: 2, Username: "user2", Country: "us", JoinedOn: TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       3,
	},
	{
		User:       domain.User{ID: 1, Username: "user1", Country: "us", JoinedOn: TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       4,
	},
}

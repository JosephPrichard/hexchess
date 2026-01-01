package svc

import (
	"hexchess-svc/db"
)

var TestUserEntities = []UserEntity{
	{
		ID:       1,
		Username: "user1",
		Country:  "us",
		Bio:      "",
		JoinedOn: db.TestTimeNow.Local(),
	},
	{
		ID:       2,
		Username: "user2",
		Country:  "us",
		Bio:      "",
		JoinedOn: db.TestTimeNow.Local(),
	},
}

var TestUserStats = []UserStatsEntity{
	{
		TotalWins:    17,
		TotalLosses:  14,
		AvgElo:       1025,
		HighestElo:   1050,
		TotalWinrate: 53,
		ModeStats: []ModeStatsEntity{
			{Mode: ModeCorrespondence7.String(), Rank: 1, Wins: 2, Losses: 2, Winrate: 50, Elo: 1000, HighestElo: 1000},
			{Mode: ModeTimed3Plus2.String(), Rank: 1, Wins: 5, Losses: 4, Winrate: 55, Elo: 1020, HighestElo: 1020},
			{Mode: ModeTimed15Plus10.String(), Rank: 1, Wins: 4, Losses: 3, Winrate: 57, Elo: 1030, HighestElo: 1030},
			{Mode: ModeTimed1Plus0.String(), Rank: 1, Wins: 6, Losses: 5, Winrate: 54, Elo: 1050, HighestElo: 1050},
		},
	},
}

var TestReplayEntities = []ReplayEntity{
	{
		ID:           1,
		WhiteID:      1,
		BlackID:      2,
		WhiteName:    "user1",
		BlackName:    "user2",
		WhiteCountry: "us",
		BlackCountry: "us",
		Mode:         ModeCorrespondence7.String(),
		Result:       WhiteWin.String(),
		Cause:        Checkmate.String(),
		WinEloDiff:   30,
		LoseEloDiff:  -30,
		WhiteElo:     1000,
		BlackElo:     1000,
		WhiteEloDiff: 30,
		BlackEloDiff: -30,
		PlayedOn:     db.TestTimeNow.Local(),
	},
	{
		ID:           3,
		WhiteID:      3,
		BlackID:      1,
		WhiteName:    "user3",
		BlackName:    "user1",
		WhiteCountry: "us",
		BlackCountry: "us",
		Mode:         ModeCorrespondence7.String(),
		Result:       Draw.String(),
		Cause:        Checkmate.String(),
		WinEloDiff:   0,
		LoseEloDiff:  0,
		WhiteElo:     900,
		BlackElo:     1000,
		WhiteEloDiff: 0,
		BlackEloDiff: 0,
		PlayedOn:     db.TestTimeNow.Local(),
	},
}

var TestChallengeEntities = []ChallengeEntity{
	{
		ChallengerID:      1,
		ChallengerName:    "user1",
		ChallengerCountry: "us",
		ChallengerElo:     1020,
		ChallengeeID:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		Mode:              ModeTimed3Plus2.String(),
		StartColor:        Random.String(),
		MadeOn:            db.TestTimeNow.Local(),
		ExpiresOn:         db.TestTimeNow.Local().Add(ExpireChallengeMaxAge),
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
		Mode:              ModeCorrespondence1.String(),
		StartColor:        Random.String(),
		MadeOn:            db.TestTimeNow.Local(),
		ExpiresOn:         db.TestTimeNow.Local().Add(ExpireChallengeMaxAge),
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
		Mode:              ModeCorrespondence1.String(),
		StartColor:        Random.String(),
		MadeOn:            db.TestTimeNow.Local(),
		ExpiresOn:         db.TestTimeNow.Local().Add(ExpireChallengeMaxAge),
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
		Mode:              ModeCorrespondence1.String(),
		StartColor:        Random.String(),
		MadeOn:            db.TestTimeNow.Local(),
		ExpiresOn:         db.TestTimeNow.Local().Add(ExpireChallengeMaxAge),
	},
}

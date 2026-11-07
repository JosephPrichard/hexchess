package tournament

import (
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/itest"
	"hexchess-svc/replay"
	"hexchess-svc/user"
	"time"
)

var TestTournaments = []Tournament{
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
		Mode:           gamedata.ModeCorrespondence1,
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
		Mode:           gamedata.ModeCorrespondence7,
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
		Mode:           gamedata.ModeCorrespondence1,
	},
	{
		ID:             4,
		TournamentKey:  itest.Tournament3ScheduledRoundRobinKey,
		Name:           "Test Tournament 3",
		Rounds:         4,
		MaxPlayerCount: -1,
		Countdown:      "10m0s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentRoundRobin,
		Status:         TournamentScheduled,
		Mode:           gamedata.ModeCorrespondence1,
	},
	{
		ID:             5,
		TournamentKey:  itest.Tournament4ScheduledSwissKey,
		Name:           "Test Tournament 4",
		Rounds:         3,
		MaxPlayerCount: -1,
		Countdown:      "11m0s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentSwiss,
		Status:         TournamentScheduled,
		Mode:           gamedata.ModeCorrespondence1,
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
		Mode:           gamedata.ModeCorrespondence1,
	},
	{
		ID:             7,
		TournamentKey:  itest.Tournament6InProgressRoundRobinKey,
		Name:           "Test Tournament 6",
		Rounds:         4,
		MaxPlayerCount: -1,
		Countdown:      "1h1m1s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentRoundRobin,
		Status:         TournamentInProgress,
		Mode:           gamedata.ModeCorrespondence1,
	},
	{
		ID:             8,
		TournamentKey:  itest.Tournament7InProgressSwissKey,
		Name:           "Test Tournament 7",
		Rounds:         3,
		MaxPlayerCount: -1,
		Countdown:      "1h2m2.002s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Ruleset:        TournamentSwiss,
		Status:         TournamentInProgress,
		Mode:           gamedata.ModeCorrespondence1,
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
		Mode:           gamedata.ModeCorrespondence1,
	},
	{
		ID:             10,
		TournamentKey:  itest.Tournament9InProgressUncompletedKey,
		Name:           "Test Tournament 9",
		Rounds:         1,
		MaxPlayerCount: -1,
		Countdown:      "1m0s",
		CreatedOn:      itest.TimeNow,
		CreatedBy:      1,
		Status:         TournamentInProgress,
		Ruleset:        TournamentSwiss,
		Mode:           gamedata.ModeCorrespondence1,
	},
}

var TestMatchesTournament9 = []FullMatch{
	{
		Ordering:      2,
		GameID:        itest.GameIDNotFinished,
		CreatedOn:     itest.TimeNow.Add(time.Minute * 1),
		TournamentKey: itest.Tournament9InProgressUncompletedKey,
		Round:         1,
		WhiteID:       10,
		BlackID:       11,
	},
	{
		Ordering:      1,
		GameID:        gameid.Identifier(itest.TournamentMatchInsts[0].GameID),
		CreatedOn:     itest.TimeNow.Add(time.Minute * 1),
		TournamentKey: itest.Tournament9InProgressUncompletedKey,
		Round:         1,
		WhiteID:       10,
		BlackID:       11,
		Replay: &TournamentReplay{
			Replay: replay.Replay{
				WhiteID:     10,
				BlackID:     11,
				Mode:        gamedata.ModeCorrespondence7,
				Result:      replay.WhiteWin,
				Cause:       replay.Checkmate,
				WinEloDiff:  30,
				LoseEloDiff: -30,
				PlayedOn:    time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			},
			ReplayColorElos: replay.ReplayColorElos{WhiteEloDiff: 30, BlackEloDiff: -30},
		},
	},
}

var TestTournamentLbdChangeSets = []struct {
	Mode    gamedata.GameMode
	ID      int64
	EloDiff float64
}{
	{gamedata.ModeCorrespondence1, 4, 1400},
	{gamedata.ModeCorrespondence1, 3, 1300},
	{gamedata.ModeCorrespondence1, 2, 1200},
	{gamedata.ModeCorrespondence1, 1, 1100},
}

// Tournament9RankedParticipants Ordered by `JoinedOn`, ranked with values in `TournamentLbdChangeSets`
var Tournament9RankedParticipants = []Participant{
	{
		User:       user.User{ID: 4, Username: "user4", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        2000,
		HighestElo: 2000,
		Rank:       1,
	},
	{
		User:       user.User{ID: 3, Username: "user3", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        900,
		HighestElo: 900,
		Rank:       2,
	},
	{
		User:       user.User{ID: 2, Username: "user2", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       3,
	},
	{
		User:       user.User{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       4,
	},
}

// Tournament8RankedParticipants Ordered by `JoinedOn`, ranked with values in `TournamentLbdChangeSets`
var Tournament8RankedParticipants = []Participant{
	{
		User:       user.User{ID: 2, Username: "user2", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       3,
	},
	{
		User:       user.User{ID: 1, Username: "user1", Country: "us", JoinedOn: itest.TimeNow},
		Elo:        1000,
		HighestElo: 1000,
		Rank:       4,
	},
}

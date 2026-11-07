package challenges

import (
	"hexchess-svc/gamedata"
	"hexchess-svc/itest"
	"time"
)

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
		Mode:              gamedata.ModeTimed3Plus2,
		StartColor:        gamedata.Random,
		MadeOn:            itest.TimeNow.Local(),
		ExpiresOn:         itest.TimeNow.Local().Add(time.Hour * 24 * 7),
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
		Mode:              gamedata.ModeCorrespondence1,
		StartColor:        gamedata.Random,
		MadeOn:            itest.TimeNow.Local(),
		ExpiresOn:         itest.TimeNow.Local().Add(time.Hour * 24 * 7),
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
		Mode:              gamedata.ModeCorrespondence1,
		StartColor:        gamedata.Random,
		MadeOn:            itest.TimeNow.Local(),
		ExpiresOn:         itest.TimeNow.Local().Add(time.Hour * 24 * 7),
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
		Mode:              gamedata.ModeCorrespondence1,
		StartColor:        gamedata.Random,
		MadeOn:            itest.TimeNow.Local(),
		ExpiresOn:         itest.TimeNow.Local().Add(time.Hour * 24 * 7),
	},
}

package network_test

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"fmt"
	"hexchess-svc/challenges"
	"hexchess-svc/game"
	gamedata2 "hexchess-svc/game/gamedata"
	"hexchess-svc/network"
	"hexchess-svc/pubsub/event"
	"hexchess-svc/tournament"
	"hexchess-svc/user"
	"time"

	"net/http"
	"testing"

	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCountEvents(t *testing.T) {
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/count", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	user.PublishActiveCountEvent(ctx, sseTest.testPubsub, 2)
	game.PublishGameCountEvent(ctx, sseTest.testPubsub, 4)

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	wantLines := []string{
		fmt.Sprintf("event: %s", network.ActiveCountEvent),
		fmt.Sprintf("data: %s", `{"kind":0,"count":0}`),
		"",
		fmt.Sprintf("event: %s", network.GamesCountEvent),
		fmt.Sprintf("data: %s", `{"kind":1,"count":3}`),
		"",
		fmt.Sprintf("event: %s", network.ActiveCountEvent),
		fmt.Sprintf("data: %s", `{"kind":0,"count":2}`),
		"",
		fmt.Sprintf("event: %s", network.GamesCountEvent),
		fmt.Sprintf("data: %s", `{"kind":1,"count":4}`),
		"",
	}

	gotEvents := scanLines(t, resp, len(wantLines))
	assert.ElementsMatch(t, wantLines, gotEvents)
}

func TestHandleActiveConn(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	sseTest := setupSSETest(t)
	defer sseTest.Close()

	broadcastSubscriber, unsubscribe, err := sseTest.testPubsub.Subscribe(ctx, event.ActiveCountChannel, "GLOBAL")
	require.NoError(t, err)
	defer unsubscribe()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/active", nil)
	req.Header.Set("Cookie", network.FmtCookie(TestSessionID1))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close() // this must execute before we assert on broadcasts since one message is sent when the SSE drops

	wantCountEvent := event.CountEvent{Kind: event.ActiveCountEvent, Count: 1}

	select {
	case message := <-broadcastSubscriber:
		var countEvent event.CountEvent
		require.NoError(t, json.Unmarshal(message, &countEvent))
		assert.Equal(t, wantCountEvent, countEvent)
	case <-ctx.Done():
		t.Errorf("test timed out: %v", ctx.Err())
	}
}

func TestHandleUserEvents(t *testing.T) {
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", network.FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	inputMessages := []user.UserEvent{
		{Kind: user.EventChallengeKind, UserID: 2, Challenge: challenges.Challenge{ChallengeeID: 2}}, // won't receive this
		{Kind: user.EventChallengeKind, UserID: 1, Challenge: challenges.Challenge{ChallengeeID: 1, Mode: gamedata2.ModeCorrespondence1, StartColor: gamedata2.White}},
		{Kind: user.EventChallengeKind, UserID: 1, Challenge: challenges.Challenge{ChallengeeID: 1, Mode: gamedata2.ModeCorrespondence7, StartColor: gamedata2.Black}},
	}

	for _, input := range inputMessages {
		user.PublishUserEvent(ctx, sseTest.testPubsub, input)
	}

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	wantLines := []string{
		fmt.Sprintf("event: %s", network.MetaEvent),
		fmt.Sprintf("data: 1"),
		"",
	}

	for _, challenge := range inputMessages[1:] {
		challengeJson, err := json.Marshal(challenge)
		require.NoError(t, err)
		wantLines = append(wantLines,
			fmt.Sprintf("event: %s", network.UserChallengeEvent),
			fmt.Sprintf("data: %s", challengeJson),
			"",
		)
	}

	gotEvents := scanLines(t, resp, len(wantLines))
	assert.ElementsMatch(t, wantLines, gotEvents)
}

func TestHandleTournamentEvents(t *testing.T) {
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Close()

	keyUUID := uuid.NewV7()
	keyString := "1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/tournament?tournamentKey="+keyString, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	// one message for each union category, with full permutations of nil/non-nil fields
	inputTournaments := []tournament.TournamentEvent{
		{Key: uuid.NewV7().String()}, // won't receive this
		{Key: keyString, Kind: tournament.TournamentStartKind},
		{Key: keyString, Kind: tournament.TournamentMatchmakingKind},
		{
			Key:  keyString,
			Kind: tournament.TournamentMatchmakingKind,
			Matches: []tournament.FullMatch{
				{
					TournamentKey: keyUUID,
					Round:         1,
					GameID:        "game-id-1",
					WhiteID:       1,
					BlackID:       2,
					CreatedOn:     time.Now(),
				},
			},
		},
		{Key: keyString, Kind: tournament.TournamentCountdownKind},
		{Key: keyString, Kind: tournament.TournamentParticipantKind},
		{
			Key:  keyString,
			Kind: tournament.TournamentParticipantKind,
			LeaderboardUser: user.LbdUser{
				User: user.User{
					ID:       1,
					Username: "John Doe",
					Country:  "us",
					JoinedOn: time.Now(),
				},
			},
		},
		{Key: keyString, Kind: tournament.TournamentErrorKind, Error: "error"},
		{Key: keyString, Kind: tournament.TournamentErrorKind},
	}

	for _, input := range inputTournaments {
		tournament.PublishTournamentEvent(ctx, sseTest.testPubsub, input)
	}

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// expect one event for each tournament input, plus the meta event
	wantLines := []string{
		fmt.Sprintf("event: %s", network.MetaEvent),
		fmt.Sprintf("data: %s", keyString),
		"",
	}
	for _, tournamentData := range inputTournaments[1:] {
		jsonData, err := json.Marshal(tournamentData)
		require.NoError(t, err)
		wantLines = append(wantLines,
			fmt.Sprintf("event: %s", network.TournamentEvent),
			fmt.Sprintf("data: %s", jsonData),
			"")
	}

	gotEvents := scanLines(t, resp, len(wantLines))
	assert.ElementsMatch(t, wantLines, gotEvents)
}

func scanLines(t *testing.T, resp *http.Response, wantLinesCount int) []string {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	defer resp.Body.Close()

	linesChan := make(chan string)

	go func() {
		// concurrently scan the SSE responses and send each line to the channel
		scan := bufio.NewScanner(resp.Body)
		defer close(linesChan)
		for scan.Scan() {
			linesChan <- scan.Text()
		}
		if err := scan.Err(); err != nil {
			t.Logf("scanner error: %v", err)
		}
	}()

	var lines []string

ScanLoop:
	for {
		var line string
		var ok bool

		// drain lines from scanner and propagate cancellation
		select {
		case <-ctx.Done():
			break ScanLoop
		case line, ok = <-linesChan:
			if ok {
				// for each event: we expect the following
				// event:\n
				// data:\n
				// \n
				lines = append(lines, line)
				if len(lines) == wantLinesCount {
					break ScanLoop
				}
			} else {
				break ScanLoop
			}
		}
	}

	t.Logf("finished scanLines loop: %v", lines)
	return lines
}

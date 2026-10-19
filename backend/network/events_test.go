package network_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/network"
	"hexchess-svc/pubsub"
	"time"

	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCountEvents(t *testing.T) {
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Shutdown()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/count", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	sseTest.broadcaster.BroadcastActiveCount(ctx, 2)
	sseTest.broadcaster.BroadcastGameCount(ctx, 4)

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	wantLines := []string{
		fmt.Sprintf("event: %s", network.ActiveCountEvent),
		fmt.Sprintf("data: %s", `{"count":0}`),
		"",
		fmt.Sprintf("event: %s", network.GamesCountEvent),
		fmt.Sprintf("data: %s", `{"count":3}`),
		"",
		fmt.Sprintf("event: %s", network.ActiveCountEvent),
		fmt.Sprintf("data: %s", `{"count":2}`),
		"",
		fmt.Sprintf("event: %s", network.GamesCountEvent),
		fmt.Sprintf("data: %s", `{"count":4}`),
		"",
	}

	gotEvents := scanLines(t, resp)
	assert.Equal(t, wantLines, gotEvents)
}

func TestHandleActiveConn(t *testing.T) {
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Shutdown()

	wantBroadcast := pubsub.GlobalCastEvent{Kind: pubsub.GlobalActiveEvent, Data: `{"count":1}`}

	broadcastSubscriber := make(chan pubsub.GlobalCastEvent, 1)
	sseTest.localBroadcasters.Counts.Subscribe(broadcastSubscriber)

	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/active", nil)
		req.Header.Set("Cookie", network.FmtCookie(TestSessionID1))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() // this must execute before we assert "wantBroadcasts" since one message is sent when the SSE drops
	}()

	var actualBroadcast pubsub.GlobalCastEvent
	select {
	case e := <-broadcastSubscriber:
		actualBroadcast = e
	case <-ctx.Done():
		t.Errorf("test timed out: %v", ctx.Err())
	}
	assert.Equal(t, wantBroadcast, actualBroadcast)
}

func TestHandleUserEvents(t *testing.T) {
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Shutdown()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", network.FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	inputMessages := []model.UserMessage{
		{Kind: model.ChallengeKind, Challenge: model.Challenge{ChallengeeID: 2}}, // won't receive this
		{Kind: model.ChallengeKind, Challenge: model.Challenge{ChallengeeID: 1, Mode: model.ModeCorrespondence1, StartColor: model.White}},
		{Kind: model.ChallengeKind, Challenge: model.Challenge{ChallengeeID: 1, Mode: model.ModeCorrespondence7, StartColor: model.Black}},
	}

	for _, input := range inputMessages {
		sseTest.broadcaster.BroadcastUserMessage(ctx, input)
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
			"")
	}

	gotEvents := scanLines(t, resp)
	assert.Equal(t, wantLines, gotEvents)
}

func TestHandleTournamentEvents(t *testing.T) {
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Shutdown()

	keyUUID := uuid.New()
	keyString := keyUUID.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/tournament?tournamentKey="+keyString, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	// one message for each union category, with full permutations of nil/non-nil fields
	inputTournaments := []model.TournamentOutput{
		{Key: uuid.NewString()}, // won't receive this
		{Key: keyString, Kind: model.TournamentStartKind},
		{Key: keyString, Kind: model.TournamentMatchmakingKind},
		{
			Key:  keyString,
			Kind: model.TournamentMatchmakingKind,
			Matches: []model.FullMatch{
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
		{Key: keyString, Kind: model.TournamentCountdownKind},
		{Key: keyString, Kind: model.TournamentParticipantKind},
		{
			Key:  keyString,
			Kind: model.TournamentParticipantKind,
			LeaderboardUser: model.LbdUser{
				User: model.User{
					ID:       1,
					Username: "John Doe",
					Country:  "us",
					JoinedOn: time.Now(),
				},
			},
		},
		{Key: keyString, Kind: model.TournamentErrorKind, Error: "error"},
		{Key: keyString, Kind: model.TournamentErrorKind},
	}

	for _, input := range inputTournaments {
		sseTest.broadcaster.BroadcastTournament(ctx, input)
	}

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// expect one event for each tournament input, plus the meta event
	wantLines := []string{
		fmt.Sprintf("event: %s", network.MetaEvent),
		fmt.Sprintf("data: %s", keyString),
		"",
	}
	for _, tournament := range inputTournaments[1:] {
		jsonData, err := json.Marshal(tournament)
		require.NoError(t, err)
		wantLines = append(wantLines,
			fmt.Sprintf("event: %s", network.TournamentEvent),
			fmt.Sprintf("data: %s", jsonData),
			"")
	}

	gotEvents := scanLines(t, resp)
	assert.Equal(t, wantLines, gotEvents)
}

func scanLines(t *testing.T, resp *http.Response) []string {
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*250)
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

	for {
		var line string
		var ok bool

		// drain lines from scanner and propagate cancellation
		select {
		case <-ctx.Done():
			return lines
		case line, ok = <-linesChan:
			if ok {
				// for each event: we expect the following
				// event:\n
				// data:\n
				// \n
				lines = append(lines, line)
			} else {
				return lines
			}
		}
	}
}

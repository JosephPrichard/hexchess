package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"time"

	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scanEvents(t *testing.T, resp *http.Response, wantEvents int) []string {
	ctx := t.Context()

	defer resp.Body.Close()

	var events []string
	if wantEvents == 0 {
		return events
	}

	var event string
	var count int

	linesChan := make(chan string)
	scan := bufio.NewScanner(resp.Body)

	go func() {
		defer close(linesChan)
		for scan.Scan() {
			linesChan <- scan.Text()
		}
	}()

	for {
		var line string
		var ok bool

		select {
		case <-ctx.Done():
			return events
		case line, ok = <-linesChan:
			if !ok {
				return events
			}
		}
		//fmt.Printf("recv line: %s\n", line)

		if line == "" {
			continue
		}
		if event == "" {
			event = line + "\n"
		} else {
			event += line + "\n"
			events = append(events, event)
			count++
			event = ""
		}
		if count >= wantEvents {
			break
		}
	}

	if err := scan.Err(); err != nil {
		t.Logf("scanner error: %v", err)
	}
	return events
}

func TestHandleCountEvents(t *testing.T) {
	t.Parallel()
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

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":3}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":2}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":4}`),
	}

	gotEvents := scanEvents(t, resp, len(wantEvents))
	assert.Equal(t, wantEvents, gotEvents)
}

func TestHandleActiveConn(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Shutdown()

	wantBroadcast := pubsub.GlobalCastEvent{Kind: pubsub.GlobalActiveEvent, Data: `{"count":1}`}

	broadcastSubscriber := make(chan pubsub.GlobalCastEvent, 1)
	sseTest.localBroadcasters.Counts.Subscribe(broadcastSubscriber)

	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/active", nil)
		req.Header.Set("Cookie", FmtCookie(TestSessionID1))
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
	t.Parallel()
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Shutdown()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(TestSessionID1))

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

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "1"),
	}

	for _, challenge := range inputMessages[1:] {
		challengeJson, err := json.Marshal(challenge)
		require.NoError(t, err)
		wantEvents = append(wantEvents, fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, challengeJson))
	}

	gotEvents := scanEvents(t, resp, len(wantEvents))
	assert.Equal(t, wantEvents, gotEvents)
}

func TestHandleTournamentEvents(t *testing.T) {
	t.Parallel()
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
	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, keyString),
	}
	for _, tournament := range inputTournaments[1:] {
		jsonData, err := json.Marshal(tournament)
		require.NoError(t, err)
		wantEvents = append(wantEvents, fmt.Sprintf("event: %s\ndata: %s\n", TournamentEvent, jsonData))
	}

	gotEvents := scanEvents(t, resp, len(wantEvents))
	assert.Equal(t, wantEvents, gotEvents)
}

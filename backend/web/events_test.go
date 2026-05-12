package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"hexchess-svc/pubsub"
	"time"

	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scanEvents(ctx context.Context, resp *http.Response, wantEvents int) []string {
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

	require.NoError(t, sseTest.broadcasters.BroadcastActiveCount(ctx, 2))
	require.NoError(t, sseTest.broadcasters.BroadcastGameCount(ctx, 1))

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":2}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":1}`),
	}

	gotEvents := scanEvents(ctx, resp, len(wantEvents))
	assert.Equal(t, wantEvents, gotEvents)
}

func TestHandleActiveConn(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Shutdown()

	wantBroadcasts := []pubsub.GlobalCastEvent{
		{Kind: pubsub.GlobalActiveEvent, Data: `{"count":1}`},
		{Kind: pubsub.GlobalActiveEvent, Data: `{"count":0}`},
	}

	broadcastSubscriber := make(chan pubsub.GlobalCastEvent, len(wantBroadcasts))
	sseTest.localBroadcasters.CountsCaster.Subscribe(broadcastSubscriber)

	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/active", nil)
		req.Header.Set("Cookie", FmtCookie(TestSessionID1))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() // this must execute before we assert "wantBroadcasts" since one message is sent when the SSE drops
	}()

	var actualBroadcasts []pubsub.GlobalCastEvent
	for range wantBroadcasts {
		select {
		case e := <-broadcastSubscriber:
			actualBroadcasts = append(actualBroadcasts, e)
		case <-ctx.Done():
			t.Errorf("test timed out: %v", ctx.Err())
		}
	}
	assert.Equal(t, wantBroadcasts, actualBroadcasts)
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

	inputChallenges := []model.Challenge{
		{ChallengeeID: 1, Mode: model.ModeCorrespondence1, StartColor: model.White},
		{ChallengeeID: 1, Mode: model.ModeCorrespondence7, StartColor: model.Black},
	}

	// broadcast all input tournaments plus one with a random tournament key (we won't receive it)
	broadcastedChallenges := []model.Challenge{
		{ChallengeeID: 2},
	}
	broadcastedChallenges = append(broadcastedChallenges, inputChallenges...)

	for _, bch := range broadcastedChallenges {
		require.NoError(t, sseTest.broadcasters.BroadcastChallenge(ctx, bch))
	}

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "1"),
	}

	for _, challenge := range inputChallenges {
		challengeJson, err := json.Marshal(challenge)
		require.NoError(t, err)
		wantEvents = append(wantEvents, fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, challengeJson))
	}

	gotEvents := scanEvents(ctx, resp, len(wantEvents))
	assert.Equal(t, wantEvents, gotEvents)
}

func TestHandleTournamentEvents(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	sseTest := setupSSETest(t)
	defer sseTest.Shutdown()

	tournamentKey := uuid.NewString()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseTest.testServer.URL+"/api/events/tournament?tournamentKey="+tournamentKey, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	// one message for each union category, with full permutations of nil/non-nil fields
	inputTournaments := []*pb.TournamentOutput{
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Start{}},
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Matchmaking{}},
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Matchmaking{
			Matchmaking: &pb.MatchmakingOutput{Matches: []*pb.TournamentMatch{
				{
					Id:            1,
					TournamentKey: tournamentKey,
					Round:         1,
					GameId:        "game-id-1",
					WhiteId:       1,
					BlackId:       2,
					CreatedOn:     time.Now().Format(time.RFC3339),
				},
			}},
		}},
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Countdown{}},
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Participant{}},
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Participant{
			Participant: &pb.LbdUser{
				Id:       1,
				Username: "John Doe",
				Country:  "us",
				JoinedOn: time.Now().Format(time.RFC3339),
			},
		}},
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Error{Error: &pb.ErrorOutput{Message: "Error"}}},
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Error{}},
	}

	// broadcast all input tournaments plus one with a random tournament key (we won't receive it)
	broadcastedTournaments := []*pb.TournamentOutput{
		{TournamentKey: uuid.NewString()},
	}
	broadcastedTournaments = append(broadcastedTournaments, inputTournaments...)

	for _, bt := range broadcastedTournaments {
		require.NoError(t, sseTest.broadcasters.BroadcastTournament(ctx, bt))
	}

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, tournamentKey),
	}
	// expect one event for each tournament input, plus the meta event
	for _, bt := range inputTournaments {
		tournamentJson, err := model.MarshalTournamentOutputJson(bt)
		require.NoError(t, err)
		wantEvents = append(wantEvents, fmt.Sprintf("event: %s\ndata: %s\n", TournamentEvent, tournamentJson))
	}

	gotEvents := scanEvents(ctx, resp, len(wantEvents))
	assert.Equal(t, wantEvents, gotEvents)
}

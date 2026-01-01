package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/db"
	"hexchess-svc/out"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"
	"net/http"
	"net/http/httptest"
	"testing"
)

// SSE tests are black box tests that connects to a given server side event, simulate the sending of messages from a producer, and checks that we receive the correct response

func scanEventsFunc(resp *http.Response, wantEvents int, fn func(string)) {
	if wantEvents == 0 {
		return
	}

	event := ""
	count := 0

	scan := bufio.NewScanner(resp.Body)
	for scan.Scan() {
		line := scan.Text()
		if line == "" {
			continue
		}
		if event == "" {
			event = line + "\n"
		} else {
			event += line + "\n"
			//fmt.Printf("sse event:%s\n", strings.ReplaceAll("\n"+event, "\n", "\n\t"))
			fn(event)
			count++
			event = ""
		}
		if count >= wantEvents {
			break
		}
	}
}

func scanEvents(resp *http.Response, wantEvents int) []string {
	var events []string
	scanEventsFunc(resp, wantEvents, func(line string) {
		events = append(events, line)
	})
	return events
}

func TestHandleCountEvents(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	setup := Setup{Redis: rdb, Broadcasters: svc.MakeBroadcaster(), EntropySource: &out.StableSource{ID: "id1"}}
	state := svc.State{Redis: rdb}

	<-setup.Broadcasters.ListenUnicastEvents(rdb)

	ts := httptest.NewServer(MakeRoot(setup))
	defer ts.Close()

	// when
	resp, err := http.Get(ts.URL + "/api/events/count")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), logutil.Trace, "broadcast-counts")
		errChan <- errors.Join(
			state.BroadcastActiveCount(ctx, 2),
			state.BroadcastGameCount(ctx, 1))
	}()

	// then
	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":2}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":1}`),
	}
	events := scanEvents(resp, len(wantEvents))
	assert.ElementsMatch(t, wantEvents, events)
}

func TestHandleUserEvents(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	setup := Setup{Redis: rdb, Broadcasters: svc.MakeBroadcaster()}
	state := svc.State{Redis: rdb}

	<-setup.Broadcasters.ListenUsersMessages(rdb)

	createTestSessions(t, rdb)

	ts := httptest.NewServer(MakeRoot(setup))
	defer ts.Close()

	// when
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	ceInput := svc.ChallengeEntity{ChallengeeID: 1, Mode: svc.ModeCorrespondence1.String(), StartColor: svc.White.String()}

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), logutil.Trace, "broadcast-user-events")
		errChan <- errors.Join(
			state.BroadcastChallenge(ctx, ceInput),
			state.BroadcastChallenge(ctx, svc.ChallengeEntity{ChallengeeID: 2}),
			state.BroadcastChallenge(ctx, ceInput))
	}()

	// then
	ceJson, err := json.Marshal(ceInput)
	require.NoError(t, err)

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "1"),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, ceJson),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, ceJson),
	}
	events := scanEvents(resp, len(wantEvents))
	assert.Equal(t, wantEvents, events)

	err = <-errChan
	require.NoError(t, err)
}

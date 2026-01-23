package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/ext"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"
	"time"

	"net/http"
	"net/http/httptest"
	"testing"
)

// SSE tests are black box tests that connect to a given server side event, simulate the sending of messages from a producer, and checks that we receive the correct response

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
	state := svc.SetupStateTest(t, itest.WithRedis)
	defer state.Close()

	state.LocalBroadcasters = svc.MakeBroadcaster()
	<-state.LocalBroadcasters.ListenUnicastEvents(state.Redis)

	ts := httptest.NewServer(MakeRoot(Setup{State: state}))
	defer ts.Close()

	// optimistic timeout incase of deadlock.
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	// when
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events/count", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), logutil.Trace, "broadcast-counts")
		errChan <- errors.Join(
			state.BroadcastActiveCount(ctx, 2),
			state.BroadcastGameCount(ctx, 1))
	}()

	// then
	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":2}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":1}`),
	}
	assert.Equal(t, wantEvents, scanEvents(resp, len(wantEvents)))
	assert.NoError(t, <-errChan)
}

func TestHandleActiveConn(t *testing.T) {
	// given
	state := svc.SetupStateTest(t, itest.WithRedis)
	defer state.Close()

	state.LocalBroadcasters = svc.MakeBroadcaster()
	state.EntropySource = &ext.StableSource{ID: "id1"}

	<-state.LocalBroadcasters.ListenUnicastEvents(state.Redis)

	wantBrdcasts := []svc.UcEvent{{Kind: svc.UcActiveEk, Data: `{"count":1}`}, {Kind: svc.UcActiveEk, Data: `{"count":0}`}}

	// optimistic timeout incase of deadlock.
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	sub := make(chan svc.UcEvent, len(wantBrdcasts))
	state.LocalBroadcasters.CountsCaster.Subscribe(sub)

	ts := httptest.NewServer(MakeRoot(Setup{State: state}))
	defer ts.Close()

	// when
	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events/active", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() // this must execute before we assert "wantBrdcasts" since one msg is sent when the SSE drops
	}()

	// then
	var brdcasts []svc.UcEvent
ReadBrdcasts:
	for range len(wantBrdcasts) {
		select {
		case e := <-sub:
			brdcasts = append(brdcasts, e)
		case <-ctx.Done():
			break ReadBrdcasts
		}
	}
	assert.Equal(t, wantBrdcasts, brdcasts)
}

func TestHandleUserEvents(t *testing.T) {
	// given
	state := svc.SetupStateTest(t, itest.WithRedis)
	defer state.Close()

	state.LocalBroadcasters = svc.MakeBroadcaster()
	<-state.LocalBroadcasters.ListenUsersMessages(state.Redis)

	createTestSessions(t, state)

	ts := httptest.NewServer(MakeRoot(Setup{State: state}))
	defer ts.Close()

	// optimistic timeout incase of deadlock.
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	// when
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	ce := svc.ChallengeEntity{ChallengeeID: 1, Mode: svc.ModeCorrespondence1.String(), StartColor: svc.White.String()}

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(t.Context(), logutil.Trace, "broadcast-user-events")
		errChan <- errors.Join(
			state.BroadcastChallenge(ctx, ce),
			state.BroadcastChallenge(ctx, svc.ChallengeEntity{ChallengeeID: 2}),
			state.BroadcastChallenge(ctx, ce))
	}()

	// then
	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	ceJson, err := json.Marshal(ce)
	require.NoError(t, err)

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "1"),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, ceJson),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, ceJson),
	}
	assert.Equal(t, wantEvents, scanEvents(resp, len(wantEvents)))
	assert.NoError(t, <-errChan)
}

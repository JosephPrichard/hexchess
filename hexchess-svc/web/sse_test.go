package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	t.Parallel()

	// given
	services := svc.SetupServicesTest(t, itest.Redis)
	defer services.Close()

	services.LocalBroadcasters = svc.MakeBroadcaster()
	<-services.LocalBroadcasters.ListenUnicastEvents(services.Redis)

	testServer := httptest.NewServer(MakeServeMux(Setup{Services: services}))
	defer testServer.Close()

	// optimistic timeout incase of deadlock.
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	// when
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/api/events/count", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), logutil.Trace, "broadcast-counts")
		errChan <- errors.Join(
			services.BroadcastActiveCount(ctx, 2),
			services.BroadcastGameCount(ctx, 1))
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
	t.Parallel()

	// given
	services := svc.SetupServicesTest(t, itest.Redis)
	defer services.Close()

	services.LocalBroadcasters = svc.MakeBroadcaster()
	services.EntropySource = &svc.StableEntropySource{ID: "id1"}

	<-services.LocalBroadcasters.ListenUnicastEvents(services.Redis)

	wantBrdcasts := []svc.UcEvent{{Kind: svc.UcActiveEk, Data: `{"count":1}`}, {Kind: svc.UcActiveEk, Data: `{"count":0}`}}

	// optimistic timeout incase of deadlock.
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	sub := make(chan svc.UcEvent, len(wantBrdcasts))
	services.LocalBroadcasters.CountsCaster.Subscribe(sub)

	testServer := httptest.NewServer(MakeServeMux(Setup{Services: services}))
	defer testServer.Close()

	// when
	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/api/events/active", nil)
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
		case <-ctx.Done():
			break ReadBrdcasts
		case e := <-sub:
			brdcasts = append(brdcasts, e)
		}
	}
	assert.Equal(t, wantBrdcasts, brdcasts)
}

func TestHandleUserEvents(t *testing.T) {
	t.Parallel()

	// given
	services := svc.SetupServicesTest(t, itest.Redis)
	defer services.Close()

	services.LocalBroadcasters = svc.MakeBroadcaster()
	<-services.LocalBroadcasters.ListenUsersMessages(services.Redis)

	createTestSessions(t, services)

	testServer := httptest.NewServer(MakeServeMux(Setup{Services: services}))
	defer testServer.Close()

	// optimistic timeout incase of deadlock.
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	// when
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	brdcastedChallenge := svc.ChallengeEntity{ChallengeeID: 1, Mode: svc.ModeCorrespondence1.String(), StartColor: svc.White.String()}

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(t.Context(), logutil.Trace, "broadcast-user-events")
		errChan <- errors.Join(
			services.BroadcastChallenge(ctx, brdcastedChallenge),
			services.BroadcastChallenge(ctx, svc.ChallengeEntity{ChallengeeID: 2}),
			services.BroadcastChallenge(ctx, brdcastedChallenge))
	}()

	// then
	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	challengeJson, err := json.Marshal(brdcastedChallenge)
	require.NoError(t, err)

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "1"),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, challengeJson),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, challengeJson),
	}
	assert.Equal(t, wantEvents, scanEvents(resp, len(wantEvents)))
	assert.NoError(t, <-errChan)
}

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

	"hexchess-svc/itest"
	"hexchess-svc/service"
	"hexchess-svc/util/logutil"

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

	services, testinfra := svc.SetupServicesTest(t, svc.ServiceMocks{}, itest.Redis)
	defer services.Close()

	broadcasters := svc.MakeLocalBroadcasters()
	<-broadcasters.ListenUnicastEvents(testinfra.Redis)

	testServer := httptest.NewServer(MakeServeMux(Setup{Services: services, Broadcasers: broadcasters}))
	defer testServer.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testServer.URL+"/api/events/count", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), logutil.Trace, "broadcast-counts")
		errChan <- errors.Join(
			services.BroadcastActiveCount(ctx, 2),
			services.BroadcastGameCount(ctx, 1))
	}()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":2}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":1}`),
	}
	assert.Equal(t, wantEvents, scanEvents(t.Context(), resp, len(wantEvents)))
	assert.NoError(t, <-errChan)
}

func TestHandleActiveConn(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	mocks := svc.ServiceMocks{
		Entropy: &svc.StableEntropySource{},
	}

	services, testinfra := svc.SetupServicesTest(t, mocks, itest.Redis)
	defer services.Close()

	broadcasters := svc.MakeLocalBroadcasters()
	<-broadcasters.ListenUnicastEvents(testinfra.Redis)

	wantBroadcasts := []svc.UcEvent{{Kind: svc.UcActiveEk, Data: `{"count":1}`}, {Kind: svc.UcActiveEk, Data: `{"count":0}`}}

	sub := make(chan svc.UcEvent, len(wantBroadcasts))
	broadcasters.CountsCaster.Subscribe(sub)

	testServer := httptest.NewServer(MakeServeMux(Setup{Services: services, Broadcasers: broadcasters}))
	defer testServer.Close()

	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/api/events/active", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() // this must execute before we assert "wantBroadcasts" since one msg is sent when the SSE drops
	}()

	var broadcasts []svc.UcEvent
	for range len(wantBroadcasts) {
		select {
		case <-ctx.Done():
			t.Errorf("test timed out: %v", ctx.Err())
		case e := <-sub:
			broadcasts = append(broadcasts, e)
		}
	}
	assert.Equal(t, wantBroadcasts, broadcasts)
}

func TestHandleUserEvents(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	services, testinfra := svc.SetupServicesTest(t, svc.ServiceMocks{}, itest.Redis)
	defer services.Close()

	broadcasters := svc.MakeLocalBroadcasters()
	<-broadcasters.ListenUsersMessages(testinfra.Redis)

	createTestSessions(t, services)

	testServer := httptest.NewServer(MakeServeMux(Setup{Services: services, Broadcasers: broadcasters}))
	defer testServer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	brdcastedChallenge := svc.Challenge{ChallengeeID: 1, Mode: svc.ModeCorrespondence1, StartColor: svc.White}

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(ctx, logutil.Trace, "broadcast-user-events")
		errChan <- errors.Join(
			services.BroadcastChallenge(ctx, brdcastedChallenge),
			services.BroadcastChallenge(ctx, svc.Challenge{ChallengeeID: 2}),
			services.BroadcastChallenge(ctx, brdcastedChallenge))
	}()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	challengeJson, err := json.Marshal(brdcastedChallenge)
	require.NoError(t, err)

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "1"),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, challengeJson),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, challengeJson),
	}
	assert.Equal(t, wantEvents, scanEvents(t.Context(), resp, len(wantEvents)))
	assert.NoError(t, <-errChan)
}

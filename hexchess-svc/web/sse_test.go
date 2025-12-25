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
	"hexchess-svc/outbound"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/services"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

	setup := RootSetup{Databases: db.Databases{Rdb: rdb}, Broadcasters: svc.MakeBroadcaster(), Generators: &outbound.MockGenerator{ID: "id1"}}
	<-svc.ListenUnicastEvents(setup.Broadcasters.CountsCaster, rdb)

	ts := httptest.NewServer(HandleRoot(setup))
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
			svc.BroadcastActiveCount(ctx, rdb, 2, "id3"),
			svc.BroadcastGameCount(ctx, rdb, 1, "id2"))
	}()

	// then
	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "id1"),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"id":"id1","count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"id":"id1","count":1}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"id":"id3","count":2}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"id":"id2","count":1}`),
	}
	events := scanEvents(resp, len(wantEvents))
	assert.ElementsMatch(t, wantEvents, events)
}

func TestHandleUserEvents(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	setup := RootSetup{Databases: db.Databases{Rdb: rdb}, Broadcasters: svc.MakeBroadcaster(), Generators: &outbound.MockGenerator{ID: "id1"}}
	<-svc.ListenUsersMessages(setup.Broadcasters.UsersCaster, rdb)

	createTestSessions(t, rdb)

	ts := httptest.NewServer(HandleRoot(setup))
	defer ts.Close()

	// when
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	ceInput := svc.ChallengeEntity{ChallengeeID: 1, Mode: svc.ModeCorrespondence1, StartColor: svc.ColorWhite}

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), logutil.Trace, "broadcast-user-events")
		errChan <- errors.Join(
			svc.BroadcastChallenge(ctx, rdb, ceInput),
			svc.BroadcastChallenge(ctx, rdb, svc.ChallengeEntity{ChallengeeID: 2}),
			svc.BroadcastChallenge(ctx, rdb, ceInput))
	}()

	// then
	ceJson, _ := json.Marshal(ceInput)

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "id1"),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, ceJson),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, ceJson),
	}
	events := scanEvents(resp, len(wantEvents))
	assert.Equal(t, wantEvents, events)

	err = <-errChan
	require.NoError(t, err)
}

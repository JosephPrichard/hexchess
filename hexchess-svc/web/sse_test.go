package web

import (
	"bufio"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func scanLines(t *testing.T, resp *http.Response, expLines int) []string {
	var lines []string

	scan := bufio.NewScanner(resp.Body)
	for scan.Scan() {
		line := scan.Text()
		t.Log(line)
		lines = append(lines, line)
		if len(lines) >= expLines {
			break
		}
	}

	return lines
}

func TestHandleCountEvents(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	state := MakeServerState(stores, nil, nil)
	data.ListenActiveCountsMessages(state.ActiveCaster, stores.Rdb.Addr)
	data.ListenGameCountsMessages(state.GamesCaster, stores.Rdb.Addr)

	ts := httptest.NewServer(HandleRoot(state))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/events/counts")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	go func() {
		ctx := context.WithValue(context.Background(), lib.TraceKey, "broadcast-counts")
		data.BroadcastMessage(ctx, stores.Rdb, data.ActiveCountChan, strconv.AppendInt(nil, 2, 10))
		data.BroadcastMessage(ctx, stores.Rdb, data.GamesCountChan, strconv.AppendInt(nil, 1, 10))
	}()

	lines := scanLines(t, resp, 5)

	expLines := []string{
		fmt.Sprintf("%s: Connected", MetaEvent),
		fmt.Sprintf("%s: 1", ActiveCountEvent),
		fmt.Sprintf("%s: 0", GamesCountEvent),
		fmt.Sprintf("%s: 2", ActiveCountEvent),
		fmt.Sprintf("%s: 1", GamesCountEvent),
	}
	assert.Equal(t, expLines, lines)
}

func TestHandleUserEvents(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	state := MakeServerState(stores, nil, nil)
	data.ListenUsersMessages(state.UsersCaster, stores.Rdb.Addr)

	sessionID := createTestSessions(t, stores.Rdb)

	ts := httptest.NewServer(HandleRoot(state))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/events/users", nil)
	assert.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(sessionID))

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	go func() {
		ctx := context.WithValue(context.Background(), lib.TraceKey, "broadcast-user-events")
		data.BroadcastChallenge(ctx, stores.Rdb, 1, data.ChallengeEntity{ChallengerID: 1})
		data.BroadcastChallenge(ctx, stores.Rdb, 2, data.ChallengeEntity{})
		data.BroadcastChallenge(ctx, stores.Rdb, 1, data.ChallengeEntity{ChallengerID: 1})
	}()

	lines := scanLines(t, resp, 3)

	json := `{"challengerId":1,"challengerName":"","challengerCountry":"","challengerElo":0,"challengeeId":0,"challengeeName":"","challengeeCountry":"","challengeeElo":0,"timeControl":0,"startColor":0,"madeOn":"0000-12-31T18:00:00-06:00"}`
	expLines := []string{
		fmt.Sprintf("%s: Connected", MetaEvent),
		fmt.Sprintf("%s: %s", UserChallengeEvent, json),
		fmt.Sprintf("%s: %s", UserChallengeEvent, json),
	}
	assert.Equal(t, expLines, lines)
}

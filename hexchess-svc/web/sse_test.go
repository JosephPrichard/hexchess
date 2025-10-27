package web

import (
	"bufio"
	"context"
	"errors"
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
	currLine := ""

	scan := bufio.NewScanner(resp.Body)
	for scan.Scan() {
		line := scan.Text()
		if line == "" {
			continue
		}
		if currLine == "" {
			currLine = line + "\n"
		} else {
			currLine += line + "\n"
			t.Log(currLine)
			lines = append(lines, currLine)
			currLine = ""
		}
		if len(lines) >= expLines {
			break
		}
	}

	return lines
}

func TestHandleCountEvents(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	state := MakeServerState(stores, nil)
	data.ListenActiveCountsMessages(state.ActiveCntCaster, stores.Rdb.PrimaryAddr)
	data.ListenGameCountsMessages(state.GamesCntCaster, stores.Rdb.PrimaryAddr)

	ts := httptest.NewServer(HandleRoot(state, ""))

	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/events/count")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), lib.TK, "broadcast-counts")
		errChan <- errors.Join(nil,
			data.BroadcastMessage(ctx, stores.Rdb, data.ActiveCountChan, strconv.AppendInt(nil, 2, 10)),
			data.BroadcastMessage(ctx, stores.Rdb, data.GamesCountChan, strconv.AppendInt(nil, 1, 10)))
	}()

	lines := scanLines(t, resp, 5)

	expLines := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "Connected"),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, "1"),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, "0"),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, "2"),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, "1"),
	}
	assert.Equal(t, expLines, lines)
}

func TestHandleUserEvents(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	state := MakeServerState(stores, nil)
	data.ListenUsersMessages(state.UsersCaster, stores.Rdb.PrimaryAddr)

	createTestSessions(t, stores.Rdb)

	ts := httptest.NewServer(HandleRoot(state, ""))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/events/user", nil)
	assert.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(sessionID1))

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), lib.TK, "broadcast-user-events")
		errChan <- errors.Join(nil,
			data.BroadcastChallenge(ctx, stores.Rdb, 1, data.ChallengeEntity{ChallengerID: 1}),
			data.BroadcastChallenge(ctx, stores.Rdb, 2, data.ChallengeEntity{ChallengerID: 2}),
			data.BroadcastChallenge(ctx, stores.Rdb, 1, data.ChallengeEntity{ChallengerID: 1}))
	}()

	lines := scanLines(t, resp, 3)

	json := `{"challengerId":1,"challengerName":"","challengerCountry":"","challengerElo":0,"challengeeId":0,"challengeeName":"","challengeeCountry":"","challengeeElo":0,"timeControl":0,"startColor":0,"madeOn":"0000-12-31T18:00:00-06:00"}`
	expLines := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "Connected"),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, json),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, json),
	}
	assert.Equal(t, expLines, lines)

	err = <-errChan
	assert.NoError(t, err)
}

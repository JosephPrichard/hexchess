package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/svc"
	"hexchess-svc/util"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func scanEventsFunc(resp *http.Response, expEvents int, fn func(string)) {
	if expEvents == 0 {
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
		if count >= expEvents {
			break
		}
	}
}

func scanEvents(resp *http.Response, expEvents int) []string {
	var events []string
	scanEventsFunc(resp, expEvents, func(line string) {
		events = append(events, line)
	})
	return events
}

func parseEventData(input string) string {
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: ")
		}
	}
	panic(fmt.Sprintf("parse event data: %s", input))
}

func TestHandleCountEvents(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()
	dbs := db.Databases{Rdb: rdb}

	state := MakeServerState(dbs, nil, "")
	state.Generators = &mockGenerator{id: "id1"}
	<-svc.ListenUnicastEvents(state.CountsCaster, dbs.Rdb)

	ts := httptest.NewServer(HandleRoot(state, ""))
	defer ts.Close()

	// when
	resp, err := http.Get(ts.URL + "/api/events/count")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), util.Trace, "broadcast-counts")
		errChan <- errors.Join(
			svc.BroadcastActiveCount(ctx, dbs.Rdb, 2, "id3"),
			svc.BroadcastGameCount(ctx, dbs.Rdb, 1, "id2"))
	}()

	// then
	expEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "id1"),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"id":"id1","count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"id":"id1","count":1}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"id":"id3","count":2}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"id":"id2","count":1}`),
	}
	events := scanEvents(resp, len(expEvents))
	assert.ElementsMatch(t, expEvents, events)
}

func TestHandleUserEvents(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()
	dbs := db.Databases{Rdb: rdb}

	state := MakeServerState(dbs, nil, "")
	state.Generators = &mockGenerator{id: "id1"}
	<-svc.ListenUsersMessages(state.UsersCaster, dbs.Rdb)

	createTestSessions(t, dbs.Rdb)

	ts := httptest.NewServer(HandleRoot(state, ""))
	defer ts.Close()

	// when
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/events/user", nil)
	assert.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Content-Type"), "text/event-stream")

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(context.Background(), util.Trace, "broadcast-user-events")
		errChan <- errors.Join(
			svc.BroadcastChallenge(ctx, dbs.Rdb, 1, svc.ChallengeEntity{ChallengerID: 1, TimeControl: svc.TcRealTime, StartColor: svc.ColorWhite}),
			svc.BroadcastChallenge(ctx, dbs.Rdb, 2, svc.ChallengeEntity{ChallengerID: 2}),
			svc.BroadcastChallenge(ctx, dbs.Rdb, 1, svc.ChallengeEntity{ChallengerID: 1, TimeControl: svc.TcRealTime, StartColor: svc.ColorWhite}))
	}()

	// then
	jsonData := `{"challengerId":1,"challengerName":"","challengerCountry":"","challengerElo":0,"challengeeId":0,"challengeeName":"","challengeeCountry":"","challengeeElo":0,"timeControl":"REAL_TIME","startColor":"WHITE","madeOn":"0001-01-01T00:00:00Z"}`
	expEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "id1"),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, jsonData),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, jsonData),
	}
	events := scanEvents(resp, len(expEvents))
	assert.ElementsMatch(t, expEvents, events)

	err = <-errChan
	assert.NoError(t, err)
}

func TestHandleCountEvents_Throughput(t *testing.T) {
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()
	dbs := db.Databases{Rdb: rdb}

	state := MakeServerState(dbs, nil, "")
	<-svc.ListenUnicastEvents(state.CountsCaster, rdb)

	ts := httptest.NewServer(HandleRoot(state, ""))
	defer ts.Close()

	runs := sseCountFlag()

	type msg struct {
		recvTime time.Time
		count    int64
	}
	type group struct {
		startTime time.Time
		subMap    map[string]msg
	}
	sseMap := make(map[string]group)

	var wg sync.WaitGroup

	for i := range runs {
		wg.Add(1)

		start := time.Now()
		resp, err := http.Get(ts.URL + "/api/events/count")
		if err != nil {
			t.Fatalf("open sse: %v", err)
		}
		// wait for initial events before we start the next SSE
		// this guarantees in the routine below, we will be listening to broadcasts from other SSE connections
		events := scanEvents(resp, 2)

		sseID := parseEventData(events[0])
		subMap := make(map[string]msg)
		sseMap[sseID] = group{startTime: start, subMap: subMap}

		// expect one message from every single SSE that connects afterward
		go func() {
			defer resp.Body.Close()
			defer wg.Done()
			scanEventsFunc(resp, runs-i, func(event string) {
				d := parseEventData(event)
				var ce svc.CountEvent
				if err := json.Unmarshal([]byte(d), &ce); err != nil {
					t.Logf("unmarshal count event: %v", err)
					return
				}
				subMap[ce.ID] = msg{recvTime: time.Now(), count: ce.Count}
			})
		}()
	}

	wg.Wait()

	count := 0
	total := time.Duration(0)

	// ordering and sseIDs are non-deterministic - prepare sorted messages by len and count content for assertions
	var results [][]int64

	for sseID, gr := range sseMap {
		var counts []int64
		for subID, m := range gr.subMap {
			counts = append(counts, m.count)
			sort.Slice(counts, func(i, j int) bool { return counts[i] < counts[j] })

			subMeta, ok := sseMap[subID]
			if ok {
				d := m.recvTime.Sub(subMeta.startTime)
				total += d
				count++
				t.Logf("%s to %s: duration: %v", sseID, subID, d)
			}
		}
		results = append(results, counts)
	}
	sort.Slice(results, func(i, j int) bool { return len(results[i]) > len(results[j]) })

	avg := time.Duration(int(total) / count)
	t.Logf("avg duration: %v", avg)

	var expected [][]int64
	for i := range runs {
		var counts []int64
		for c := i + 1; c <= runs; c++ {
			counts = append(counts, int64(c))
		}
		expected = append(expected, counts)
	}

	assert.Len(t, results, len(expected))
	for i := range expected {
		assert.Equal(t, expected[i], results[i])
	}
}

package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"hexchess-svc/service"
	// "time"

	"net/http"
	"net/http/httptest"
	"testing"

	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"

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

	services, testinfra := svc.SetupServicesTest(t, svc.Mocks{}, itest.Redis)
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

	require.NoError(t, <-errChan)

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":0}`),
		fmt.Sprintf("event: %s\ndata: %s\n", ActiveCountEvent, `{"count":2}`),
		fmt.Sprintf("event: %s\ndata: %s\n", GamesCountEvent, `{"count":1}`),
	}
	assert.Equal(t, wantEvents, scanEvents(t.Context(), resp, len(wantEvents)))
}

func TestHandleActiveConn(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	mocks := svc.Mocks{Entropy: &svc.StableEntropySource{}}

	services, testinfra := svc.SetupServicesTest(t, mocks, itest.Redis)
	defer services.Close()
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

	services, testinfra := svc.SetupServicesTest(t, svc.Mocks{}, itest.Redis)
	defer services.Close()

	broadcasters := svc.MakeLocalBroadcasters()
	<-broadcasters.ListenUsersMessages(testinfra.Redis)

	testServer := httptest.NewServer(MakeServeMux(Setup{Services: services, Broadcasers: broadcasters}))
	defer testServer.Close()

	createTestSessions(t, services)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/api/events/user", nil)
	require.NoError(t, err)
	req.Header.Set("Cookie", FmtCookie(TestSessionID1))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	broadcastedChallenge := model.Challenge{ChallengeeID: 1, Mode: model.ModeCorrespondence1, StartColor: model.White}

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(ctx, logutil.Trace, "broadcast-user-events")
		errChan <- errors.Join(
			services.BroadcastChallenge(ctx, broadcastedChallenge),
			services.BroadcastChallenge(ctx, model.Challenge{ChallengeeID: 2}),
			services.BroadcastChallenge(ctx, broadcastedChallenge))
	}()

	require.NoError(t, <-errChan)

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	challengeJson, err := json.Marshal(broadcastedChallenge)
	require.NoError(t, err)

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, "1"),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, challengeJson),
		fmt.Sprintf("event: %s\ndata: %s\n", UserChallengeEvent, challengeJson),
	}
	assert.Equal(t, wantEvents, scanEvents(t.Context(), resp, len(wantEvents)))
}

func TestHandleTournamentEvents(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	services, testinfra := svc.SetupServicesTest(t, svc.Mocks{}, itest.Redis)
	defer services.Close()

	broadcasters := svc.MakeLocalBroadcasters()
	<-broadcasters.ListenTournamentMessages(testinfra.Redis)

	testServer := httptest.NewServer(MakeServeMux(Setup{Services: services, Broadcasers: broadcasters}))
	defer testServer.Close()

	tournamentKey := uuid.NewString();

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/api/events/tournament?tournamentKey="+tournamentKey, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	broadcastTournaments := []*pb.TournamentOutput{
		{TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Start{}},
		// {TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Matchmaking{}},
		// {TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Countdown{}},
		// {TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Participant{
		// 	Participant: &pb.LbdUser{
		// 		Id: 1,
		// 		Username: "John Doe",
		// 		JoinedOn: time.Now().Format(time.RFC3339),
		// 	},
		// }},
		// {TournamentKey: tournamentKey, Value: &pb.TournamentOutput_Error{}},
	}

	errChan := make(chan error)
	go func() {
		ctx := context.WithValue(ctx, logutil.Trace, "broadcast-tournaments")

		broadcastTournaments = append(broadcastTournaments,
			&pb.TournamentOutput{TournamentKey: uuid.NewString()},
		)

		var errs []error
		for _, bt := range broadcastTournaments {
			errs = append(errs, services.BroadcastTournament(ctx, bt))
		}

		errChan <- errors.Join(errs...)
	}()

	require.NoError(t, <-errChan)

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	wantEvents := []string{
		fmt.Sprintf("event: %s\ndata: %s\n", MetaEvent, tournamentKey),
	}

	for _, bt := range broadcastTournaments {
		tournamentJson, err := svc.MarshalTournamentOutputJson(bt)
		require.NoError(t, err)
		wantEvents = append(wantEvents, fmt.Sprintf("event: %s\ndata: %s\n", TournamentEvent, tournamentJson))
	}

	assert.Equal(t, wantEvents, scanEvents(t.Context(), resp, len(wantEvents)))
}
import http from "k6/http";
import { Trend } from "k6/metrics";
import faker from "k6/x/faker";
import { URLSearchParams } from "https://jslib.k6.io/url/1.0.0/index.js";

// Constant VU definitions, test configs and input parsing

const hostname = __ENV.HOSTNAME || "http://localhost:8081";
export const baseUri = `${hostname}/api`;

// providing a higher user count gives a better distribution on what data is created and which rows are updated
// higher is better, but takes much longer to run the test
const usersCount = parseInt(__ENV.USERS_COUNT) || 10;

export const constantArrivalRate = {
    executor: "constant-arrival-rate",
    rate: 1,
    timeUnit: "1s",
    duration: "5s",
    preAllocatedVUs: 1,
    maxVUs: 2,
};

// export const constantArrivalRate = {
//     executor: "constant-arrival-rate",
//     rate: 20,
//     timeUnit: "1s",
//     duration: "1m",
//     preAllocatedVUs: 10,
//     maxVUs: 100,
// };

export const options = {
    scenarios: {
        // GET testdefs
        // leaderboard
        get_leaderboard: {
            ...constantArrivalRate,
            exec: "getLeaderboard",
        },
        // replays
        search_replays: {
            ...constantArrivalRate,
            exec: "searchReplays",
        },
        search_replays_winner_loser_id: {
            ...constantArrivalRate,
            exec: "searchReplays_WinnerID_LoserID",
        },
        search_replays_black_white_id: {
            ...constantArrivalRate,
            exec: "searchReplays_WhiteID_BlackID",
        },
        search_replays_timeframe: {
            ...constantArrivalRate,
            exec: "searchReplays_Timeframe",
        },
        search_replays_mode_result_cause: {
            ...constantArrivalRate,
            exec: "searchReplays_ModeResultCause",
        },
        // users
        search_users: {
            ...constantArrivalRate,
            exec: "searchUsers",
        },
        // challenges
        get_challenges: {
            ...constantArrivalRate,
            exec: "getChallenges",
        },
        // game metadata
        get_game_metadatas: {
            ...constantArrivalRate,
            exec: "getGameMetadatas",
        },
        // replay elo histories
        get_elo_histories: {
            ...constantArrivalRate,
            exec: "getEloHistories",
        },
        // POST testdefs
        // users
        update_user: {
            ...constantArrivalRate,
            exec: "postUpdateUser"
        },
        // challenges
        create_challenge: {
            ...constantArrivalRate,
            exec: "postCreateChallenge",
        },
        // update_challenge: {
        //     ...constantArrivalRate,
        //     exec: "postUpdateChallenge",
        // },
        // games
        create_game: {
            ...constantArrivalRate,
            exec: "postCreateGame",
        }
    },
};

// Test preconditions, setup, and teardown

export function setup() {
    // k6 does not support async/await so we use a single batch to send all the requests at once.
    const requests = [];
    for (let i = 0; i < usersCount; i++) {
        // precondition: seeded data is assumed to have usernames conforming to this pattern, each with the provided password
        requests.push(["POST", baseUri + "/login", JSON.stringify({ username: `User${i}`, password: "password1" })]);
    }
    
    const responses = http.batch(requests);

    // these session cookies can be used anytime we need an authenticated request.
    // a big assumption is: these stay valid until the end of the test
    const sessionCookies = responses.map(resp => {
        // console.log(`Login response headers=${JSON.stringify(resp.headers)}, status=${resp.status}, body=${resp.body}`);
        return resp.headers["Set-Cookie"];
    });

    console.log("Generated session cookies in test setup", sessionCookies);

    return { sessionCookies };
}

// Additional measurements

export const trendCreateChallenge400 = new Trend("create_challenge_400_duration");

// GET test implementations

export function getLeaderboard() {
    const params = new URLSearchParams({ mode: randArray(gameModes) });
    http.get(baseUri + "/leaderboard?" + params.toString());
}

export function getTournaments() {
    http.get(baseUri + "/tournaments");
}

export function getTournament() {
    const params = new URLSearchParams({ tournamentKey: randTournamentKey() });
    http.get(baseUri + "/tournament?" + params.toString());
}

export function getGameMetadatas() {
    http.get(baseUri + "/game/rooms");
}

export function getChallenges(data) {
    const searchParams = new URLSearchParams({
        participants: randrange(0, 1) === 0 ? "sent" : "received"
    });
    const headerParams = makeSessionHeaders(data.sessionCookies);
    http.get(baseUri + "/challenges?" + searchParams.toString(), headerParams);
}

export function searchReplays() {
    http.get(baseUri + "/replays");
}

export function searchReplays_WinnerID_LoserID() {
    const params = new URLSearchParams({ 
        winnerId: randUserID(), 
        loserId: randUserID() 
    });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_WhiteID_BlackID() {
    const params = new URLSearchParams({
        whiteId: randUserID(), 
        blackId: randUserID() 
    });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_Timeframe() {
    const params = new URLSearchParams({});
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_ModeResultCause() {
    const params = new URLSearchParams({ 
        mode: randArray(gameModes), 
        cause: randArray(replayCauses), 
        result: randArray(replayResults),
    });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchUsers() {
    const params = new URLSearchParams({ username: "John" });
    http.get(baseUri + "/players/search?" + params.toString());
}

export function getEloHistories() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUri + "/replay/elo-histories?" + params.toString());
}

// POST test implementations

export function postCreateGame() {
    const body = JSON.stringify({
        mode: randArray(gameModes),
        firstColor: randArray(colors),
    });
    http.post(baseUri + "/games/create", body);
}

export function postCreateChallenge(data) {
    const body = JSON.stringify({
        challengeeId: randUserIDInt(),
        mode: randArray(gameModes),
        startColor: randArray(colors),
    });
    const params = makeSessionHeaders(data.sessionCookies);

    // reasonable chance of returning an error if this duplicate or self challenge
    const resp = http.post(baseUri + "/challenges/create", body, params);
    if (resp.status === 400) {
        trendCreateChallenge400.add(resp.timings.duration);
    }
}

export function postUpdateChallenge(data) {
    const body = JSON.stringify({});
    const params = makeSessionHeaders(data.sessionCookies);

    const resp = http.post(baseUri + "/challenges/update", body, params);
}

export function postUpdateUser(data) {
    const body = JSON.stringify({ 
        // we're avoiding updating the username as to keep the username preconditions stable for the next perf test execution.
        // bio is the bulk of the data that is updated, and is sufficient to test this endpoint.
        newCountry: "us",
        newBio: faker.word.loremIpsumParagraph(1, 3, 8, "\n"),
    });
    const params = makeSessionHeaders(data.sessionCookies);

    http.post(baseUri + "/users", body, params);
}

// Utility functions for test impls

function uuidFromBigInt(bigint) {
  // convert to 128-bit hex string (32 hex chars, zero-padded)
  const hex = bigint.toString(16).padStart(32, '0');

  // insert dashes in the 8-4-4-4-12 UUID format
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20, 32),
  ].join('-');
}

// Utilities for generating test inputs

function makeSessionHeaders(sessionCookies) {
    const cookie = randArray(sessionCookies);
    return { 
        headers: { "Cookie": cookie }
    };
}

const minUser = 1;
const maxUser = 1000;

// seed script will generate a sequence of continously increasing uuids, starting from 0, so we can safely randrange a tournament key
const minTournamentKey = 1;
const maxTournamentKey = 250;

function randrange(min, max) {
    return Math.floor(Math.random() * (max - min + 1)) + min; // inclusive range (min, max)
}

function randUserID() {
    return String(randUserIDInt());
}

function randUserIDInt() {
    return randrange(minUser, maxUser);
}

function randTournamentKey() {
    return randrange(minTournamentKey, maxTournamentKey);
}

const gameModes = [
    "CORRESPONDENCE_1",
    "CORRESPONDENCE_7",
    "CORRESPONDENCE_14",
    "TIMED_1+0",
    "TIMED_3+2",
    "TIMED_15+10"
];

const replayCauses = [
    "CHECKMATE",
    "STALEMATE",
    "FORFEIT"
];

const replayResults = [
    "WHITE_WINS",
    "BLACK_WINS",
    "DRAW",
];

const colors = [
    "WHITE",
    "BLACK",
    "RANDOM"
];

function randArray(array) {
    return array[randrange(0, array.length - 1)];
}
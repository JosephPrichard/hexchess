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
const gamesCount = parseInt(__ENV.GAMES_COUNT) || 100;

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
        get_replay_replay_id: {
            ...constantArrivalRate,
            exec: "getReplay_ReplayID",
        },
        get_replay_movelist: {
            ...constantArrivalRate,
            exec: "getReplayMoveList",
        },
        // players / users
        search_players: {
            ...constantArrivalRate,
            exec: "searchPlayers",
        },
        get_player: {
            ...constantArrivalRate, 
            exec: "getPlayer",
        },
        get_self_player: {
             ...constantArrivalRate, 
            exec: "getSelfPlayer",
        },
        get_player_activity: {
            ...constantArrivalRate,
            exec: "getPlayerActivity",
        },
        get_profile_pic: {
            ...constantArrivalRate,
            exec: "getProfilePic",
        },
        // game /rooms
        get_game_rooms: {
            ...constantArrivalRate,
            exec: "getGameRooms",
        },
        get_game_chats: {
            ...constantArrivalRate,
            exec: "getGameChats",
        },
        get_game_room_exists: {
            ...constantArrivalRate,
            exec: "getGameRoomExists",
        },
        // tournaments
        get_tournament: {
            ...constantArrivalRate,
            exec: "getTournament",
        },
        get_tournaments: {
            ...constantArrivalRate,
            exec: "getTournaments",
        },
        // challenges
        get_challenges: {
            ...constantArrivalRate,
            exec: "getChallenges",
        },
        get_challenges_count: {
            ...constantArrivalRate,
            exec: "getChallengesCount",
        },
        // replay elo histories
        get_elo_histories: {
            ...constantArrivalRate,
            exec: "getEloHistories",
        },
        // POST testdefs
        // session
        create_temp_session: {
            ...constantArrivalRate,
            exec: "postCreateTempSession",
        },
        refresh_session: {
            ...constantArrivalRate,
            exec: "postRefreshSession"
        },
        // users
        update_user: {
            ...constantArrivalRate,
            exec: "postUpdateUser",
        },
        // upload_profile_pic: {
        //     ...constantArrivalRate,
        //     exec: "postUploadProfilePic",
        // },
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
    const sessionRequests = [];
    const gameRequests = [];

    // we need to seed sessions for authenticated requests 
    for (let i = 0; i < usersCount; i++) {
        // precondition: seeded data is assumed to have usernames conforming to this pattern, each with the provided password
        const body = JSON.stringify({ username: `User${i}`, password: "password1" });
        sessionRequests.push(["POST", baseUri + "/login", body]);
    }

    // create some games beforehand (seperate from the ones created as part of resting the /game/create endpoint) for game data reads
    for (let i = 0; i < gamesCount; i++) {
        const body = JSON.stringify({ mode: pickElement(gameModes), firstColor: pickElement(colors) });
        gameRequests.push(["POST", baseUri + "/games/create", body]);
    }
    
    const sessionRepsonses = http.batch(sessionRequests);
    const gameResponses = http.batch(gameRequests);

    // these session cookies can be used anytime we need an authenticated request.
    // a big assumption is: these stay valid until the end of the test
    const sessionCookies = sessionRepsonses.map(resp => {
        // console.log(`Login response headers=${JSON.stringify(resp.headers)}, status=${resp.status}, body=${resp.body}`);
        return resp.headers["Set-Cookie"];
    });


    const gameIds = gameResponses.map(resp => {
        // console.log(`Create game response status=${resp.status}, body=${resp.body}`);
        return JSON.parse(resp.body).gameId;
    });

    return { sessionCookies, gameIds };
}

// Additional measurements

export const trendCreateChallenge400 = new Trend("create_challenge_400_duration");

// GET test implementations

// leaderboard

export function getLeaderboard() {
    const params = new URLSearchParams({ mode: pickElement(gameModes) });
    http.get(baseUri + "/leaderboard?" + params.toString());
}

// tournament

export function getTournaments() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUri + "/tournaments?" + params.toString());
}

export function getTournament() {
    const params = new URLSearchParams({ tournamentKey: randTournamentKey() });
    http.get(baseUri + "/tournament?" + params.toString());
}

// challenges

export function getChallenges(data) {
    const searchParams = new URLSearchParams({
        participants: randrange(0, 1) === 0 ? "sent" : "received"
    });
    const headerParams = makeSessionHeaders(data.sessionCookies);
    http.get(baseUri + "/challenges?" + searchParams.toString(), headerParams);
}

export function getChallengesCount(data) {
    const headerParams = makeSessionHeaders(data.sessionCookies);
    http.get(baseUri + "/challenges/count", headerParams);
}

// replays

export function getReplay_ReplayID() {
    const searchParams = new URLSearchParams({
        id: randReplayID()
    });
    http.get(baseUri + "/replay?" + searchParams.toString());
}

export function getReplayMoveList(data) {
    const params = new URLSearchParams({ replayId: randReplayID() });
    http.get(baseUri + "/replay/move-list?" + params.toString());
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
    // TODO: find a way to have the seeded data have stable timeframes
    const params = new URLSearchParams({
        fromDate: randDate(2025),
        toDate: randDate(2026),
    });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_ModeResultCause() {
    const params = new URLSearchParams({ 
        mode: pickElement(gameModes), 
        cause: pickElement(replayCauses), 
        result: pickElement(replayResults),
    });
    http.get(baseUri + "/replays?" + params.toString());
}

// players / users

export function getPlayer() {
    const params = new URLSearchParams({ id: randUserID() });
    http.get(baseUri + "/players?" + params.toString());
}

export function getSelfPlayer() {
    const headerParams = makeSessionHeaders(data.sessionCookies);
    http.get(baseUri + "/players/self", headerParams);
}

// TODO: have some of the players be active so this returns something other than false.
export function getPlayerActivity() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUri + "/players/activity?" + params.toString());
}

export function getProfilePic() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUri + "/users/profile-pics?" + params.toString());
}

export function searchPlayers() {
    const params = new URLSearchParams({ username: "John" });
    http.get(baseUri + "/players/search?" + params.toString());
}

export function getEloHistories() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUri + "/replay/elo-histories?" + params.toString());
}

// game / rooms

export function getGameRooms() {
    const params = makeSessionHeaders(data.sessionCookies);
    http.get(baseUri + "/game/rooms", params);
}

export function getGameChats(data) {
    const params = new URLSearchParams({ gameId: pickElement(data.gameIds) });
    http.get(baseUri + "/game/rooms/chats?" + params.toString());
}

export function getGameRoomExists(data) {
    const params = new URLSearchParams({ gameId: pickElement(data.gameIds) });
    http.get(baseUri + "/game/rooms/exists?" + params.toString());
}

// POST test implementations

// session

export function postCreateTempSession() {
    const params = makeSessionHeaders(data.sessionCookies);
    http.post(baseUri + "/session/temp", null, params);
}

export function postRefreshSession() {
    const params = makeSessionHeaders(data.sessionCookies);
    http.post(baseUri + "/session/refresh", null, params);
}

// game

export function postCreateGame() {
    const body = JSON.stringify({
        mode: pickElement(gameModes),
        firstColor: pickElement(colors),
    });
    http.post(baseUri + "/games/create", body);
}

// challenges

export function postCreateChallenge(data) {
    const body = JSON.stringify({
        challengeeId: randUserIDInt(),
        mode: pickElement(gameModes),
        startColor: pickElement(colors),
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

// user / player

const inputFile = open("./input-image.png", "b");

export function postUploadProfilePic(data) {
    const params = makeSessionHeaders(data.sessionCookies);
    params.headers["Content-Type"] = "image/png";

    http.post(baseUri + "/users/profile-pics", inputFile, params);
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
  const hex = bigint.toString(16).padEnd(32, '0');

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
    const cookie = pickElement(sessionCookies);
    return { 
        headers: { "Cookie": cookie }
    };
}

const minUserID = 1;
const maxUserID = 1000;

// seed script will generate a sequence of continously increasing uuids, starting from 0, so we can safely randrange a tournament key
const minTournamentKey = 1;
const maxTournamentKey = 250;

const minReplayID = 1;
const maxReplayID = 5000;

function randrange(min, max) {
    return Math.floor(Math.random() * (max - min + 1)) + min; // inclusive range (min, max)
}

function randUserID() {
    return String(randUserIDInt());
}

function randUserIDInt() {
    return randrange(minUserID, maxUserID);
}

function randTournamentKey() {
    const tkeyint = randrange(minTournamentKey, maxTournamentKey);
    return uuidFromBigInt(tkeyint);
}

function randReplayID() {
    return randrange(minReplayID, maxReplayID);
}

function randDate(year) {
    const days = String(randrange(1, 28)).padStart(2, '0');
    const month = String(randrange(1, 12)).padStart(2, '0');
    return `${String(year)}-${month}-${days}`;
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

function pickElement(array) {
    return array[randrange(0, array.length - 1)];
}
import http, { expectedStatuses } from "k6/http";
import { Trend } from "k6/metrics";
import faker from "k6/x/faker";
import { URLSearchParams } from "https://jslib.k6.io/url/1.0.0/index.js";
import { setupSessions, setupGames } from "./setup.js"
import { 
    makeSessionParams, 
    randrange, 
    randUserID, 
    randUserIDInt, 
    randTournamentKey, 
    randReplayID, 
    randDate, 
    replayCauses, 
    replayResults, 
    gameModes, 
    colors, 
    pickElement,
    pickSession
} from "./utils.js"

// Constant VU definitions, test configs and input parsing

const protocol = __ENV.PROTOCOL || "http";
const hostname = __ENV.HOSTNAME || "localhost:8081";
export const baseUrl = `${protocol}://${hostname}/api`;

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
        upload_profile_pic: {
            ...constantArrivalRate,
            exec: "postUploadProfilePic",
        },
        // challenges
        create_challenge: {
            ...constantArrivalRate,
            exec: "postCreateChallenge",
        },
        update_challenge: {
            ...constantArrivalRate,
            exec: "postUpdateChallenge",
        },
        // games
        create_game: {
            ...constantArrivalRate,
            exec: "postCreateGame",
        }
    },
};

// Test preconditions, setup, and teardown

export function setup() {
    const sessions = setupSessions(usersCount);
    const gameIds = setupGames(gamesCount);
    return { sessions, gameIds };
}

// Additional measurements

export const trendCreateChallenge400 = new Trend("create_challenge_400_duration");
export const trendUpdateChallenge404 = new Trend("update_challenge_404_duration");

// GET test implementations

// leaderboard

export function getLeaderboard() {
    const params = new URLSearchParams({ mode: pickElement(gameModes) });
    http.get(baseUrl + "/leaderboard?" + params.toString());
}

// tournament

export function getTournaments() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUrl + "/tournaments?" + params.toString());
}

export function getTournament() {
    const params = new URLSearchParams({ tournamentKey: randTournamentKey() });
    http.get(baseUrl + "/tournament?" + params.toString());
}

// challenges

export function getChallenges(data) {
    const searchParams = new URLSearchParams({
        participants: randrange(0, 1) === 0 ? "sent" : "received"
    });
    const headerParams = makeSessionParams(data.sessions);
    http.get(baseUrl + "/challenges?" + searchParams.toString(), headerParams);
}

export function getChallengesCount(data) {
    const headerParams = makeSessionParams(data.sessions);
    http.get(baseUrl + "/challenges/count", headerParams);
}

// replays

export function getReplay_ReplayID() {
    const searchParams = new URLSearchParams({
        id: randReplayID()
    });
    http.get(baseUrl + "/replay?" + searchParams.toString());
}

export function getReplayMoveList(data) {
    const params = new URLSearchParams({ replayId: randReplayID() });
    http.get(baseUrl + "/replay/move-list?" + params.toString());
}

export function searchReplays() {
    http.get(baseUrl + "/replays");
}

export function searchReplays_WinnerID_LoserID() {
    const params = new URLSearchParams({ 
        winnerId: randUserID(), 
        loserId: randUserID() 
    });
    http.get(baseUrl + "/replays?" + params.toString());
}

export function searchReplays_WhiteID_BlackID() {
    const params = new URLSearchParams({
        whiteId: randUserID(), 
        blackId: randUserID() 
    });
    http.get(baseUrl + "/replays?" + params.toString());
}

export function searchReplays_Timeframe() {
    // TODO: find a way to have the seeded data have stable timeframes
    const params = new URLSearchParams({
        fromDate: randDate(2025),
        toDate: randDate(2026),
    });
    http.get(baseUrl + "/replays?" + params.toString());
}

export function searchReplays_ModeResultCause() {
    const params = new URLSearchParams({ 
        mode: pickElement(gameModes), 
        cause: pickElement(replayCauses), 
        result: pickElement(replayResults),
    });
    http.get(baseUrl + "/replays?" + params.toString());
}

// players / users

export function getPlayer() {
    const params = new URLSearchParams({ id: randUserID() });
    http.get(baseUrl + "/players?" + params.toString());
}

export function getSelfPlayer(data) {
    const headerParams = makeSessionParams(data.sessions);
    http.get(baseUrl + "/players/self", headerParams);
}

// TODO: have some of the players be active so this returns something other than false.
export function getPlayerActivity() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUrl + "/players/activity?" + params.toString());
}

export function getProfilePic() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUrl + "/users/profile-pics?" + params.toString());
}

export function searchPlayers() {
    const params = new URLSearchParams({ username: "John" });
    http.get(baseUrl + "/players/search?" + params.toString());
}

export function getEloHistories() {
    const params = new URLSearchParams({ userId: randUserID() });
    http.get(baseUrl + "/replay/elo-histories?" + params.toString());
}

// game / rooms

export function getGameRooms(data) {
    const params = makeSessionParams(data.sessions);
    http.get(baseUrl + "/game/rooms", params);
}

export function getGameChats(data) {
    const params = new URLSearchParams({ gameId: pickElement(data.gameIds) });
    http.get(baseUrl + "/game/rooms/chats?" + params.toString());
}

export function getGameRoomExists(data) {
    const params = new URLSearchParams({ gameId: pickElement(data.gameIds) });
    http.get(baseUrl + "/game/rooms/exists?" + params.toString());
}

// POST test implementations

// session

export function postCreateTempSession(data) {
    const params = makeSessionParams(data.sessions);
    http.post(baseUrl + "/session/temp", null, params);
}

export function postRefreshSession(data) {
    const params = makeSessionParams(data.sessions);
    http.post(baseUrl + "/session/refresh", null, params);
}

// game

export function postCreateGame() {
    const body = JSON.stringify({
        mode: pickElement(gameModes),
        firstColor: pickElement(colors),
    });
    http.post(baseUrl + "/games/create", body);
}

// challenges

export function postCreateChallenge(data) {
    const body = JSON.stringify({
        challengeeId: randUserIDInt(),
        mode: pickElement(gameModes),
        startColor: pickElement(colors),
    });
    const params = makeSessionParams(data.sessions);

    params.responseCallback = expectedStatuses(200, 400); // 400 could be a duplicate or self challenge, which are not classified as perf test failures

    const resp = http.post(baseUrl + "/challenges/create", body, params);
    if (resp.status === 400) {
        trendCreateChallenge400.add(resp.timings.duration);
    }
}

export function postUpdateChallenge(data) {
    const [params, userId] = pickSession(data.sessions);
    const body = JSON.stringify({
        challengerID: randUserIDInt(),
        challengeeID: userId,
        action: "ACCEPT",
    });

    params.responseCallback = expectedStatuses(200, 404); // challenge may not exist, this is expected

    const resp = http.post(baseUrl + "/challenges/update", body, params);
     if (resp.status === 404) {
        trendUpdateChallenge404.add(resp.timings.duration);
    }
}

// user / player

const inputFile = open("./inputs/sample-image.png", "b");
const inputFileChecksum = "q3ayd2OBXomVIgArjuf4rkuvzYDh2Ju2rxEWq1zCuCM="; // hardcoded, you must change this if you change the input file

export function postUploadProfilePic(data) {
    const params = makeSessionParams(data.sessions);

    params.headers["Content-Type"] = "image/png";
    params.headers["Content-Digest"] = inputFileChecksum;
    params.headers["Content-Length"] = inputFile.byteLength;

    http.post(baseUrl + "/users/profile-pics", inputFile, params);
}

export function postUpdateUser(data) {
    const body = JSON.stringify({ 
        // we're avoiding updating the username as to keep the username preconditions stable for the next perf test execution.
        // bio is the bulk of the data that is updated, and is sufficient to test this endpoint.
        newCountry: "us",
        newBio: faker.word.loremIpsumParagraph(1, 3, 8, "\n"),
    });
    const params = makeSessionParams(data.sessions);

    http.post(baseUrl + "/users", body, params);
}
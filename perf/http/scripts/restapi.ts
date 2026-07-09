import http, { expectedStatuses } from "k6/http";
import { Trend } from "k6/metrics";
import { setupSessions, setupGames, Session } from "./setup.ts";
import { 
    pickSessionParams, 
    randrange, 
    randUserID, 
    randUserIDInt,
    randReplayID, 
    randDate, 
    replayCauses, 
    replayResults, 
    gameModes, 
    colors, 
    pickElement,
    pickSession
} from "./testdata.ts";

// ignore typechecking for CDN imports and K6 extensions
// @ts-ignore
import faker from "k6/x/faker";
// @ts-ignore
import { URLSearchParams } from "https://jslib.k6.io/url/1.0.0/index.js";

// VU definitions, test configs and input parsing

const protocol = __ENV.PROTOCOL ?? "http";
const hostname = __ENV.HOSTNAME ?? "localhost:8081";
export const baseUrl = `${protocol}://${hostname}/api`;

// providing a higher user/games count gives a better distribution on what data is created and which rows are updated
const usersCount = parseInt(__ENV.USERS_COUNT) || 10;
const gamesCount = parseInt(__ENV.GAMES_COUNT) || 100;

// pick a tournamnt key out of the database.
const staticTournamentKey = __ENV.TOURNAMENT_KEY || "missing";

const profile = __ENV.PROFILE || "smoke";

const configs: Record<string, any> = {
    "smoke": {
        executor: "constant-arrival-rate",
        rate: 1,
        timeUnit: "1s",
        duration: "5s",
        preAllocatedVUs: 1,
        maxVUs: 2,
    },
    "capacity": {
        executor: "constant-arrival-rate",
        rate: 20,
        timeUnit: "1s",
        duration: "1m",
        preAllocatedVUs: 10,
        maxVUs: 100,
    }
};

export const options = {
    scenarios: {
        // GET testdefs
        // leaderboard
        getLeaderboard: {
            ...configs[profile],
            exec: "getLeaderboard",
        },
        // replays
        searchReplays: {
            ...configs[profile],
            exec: "searchReplays",
        },
        searchReplaysWinnerIDLoserID: {
            ...configs[profile],
            exec: "searchReplaysWinnerIDLoserID",
        },
        searchReplaysWhiteIDBlackID: {
            ...configs[profile],
            exec: "searchReplaysWhiteIDBlackID",
        },
        searchReplaysTimeframe: {
            ...configs[profile],
            exec: "searchReplaysTimeframe",
        },
        searchReplaysModeResultCause: {
            ...configs[profile],
            exec: "searchReplaysModeResultCause",
        },
        getReplay_ReplayID: {
            ...configs[profile],
            exec: "getReplay_ReplayID",
        },
        getReplayMoveList: {
            ...configs[profile],
            exec: "getReplayMoveList",
        },
        // players / users
        searchPlayers: {
            ...configs[profile],
            exec: "searchPlayers",
        },
        getPlayer: {
            ...configs[profile], 
            exec: "getPlayer",
        },
        getSelfPlayer: {
             ...configs[profile], 
            exec: "getSelfPlayer",
        },
        getPlayerActivity: {
            ...configs[profile],
            exec: "getPlayerActivity",
        },
        getProfilePic: {
            ...configs[profile],
            exec: "getProfilePic",
        },
        // game /rooms
        getGameRooms: {
            ...configs[profile],
            exec: "getGameRooms",
        },
        getGameChats: {
            ...configs[profile],
            exec: "getGameChats",
        },
        getGameRoomExists: {
            ...configs[profile],
            exec: "getGameRoomExists",
        },
        // tournaments
        getTournament: {
            ...configs[profile],
            exec: "getTournament",
        },
        getTournaments: {
            ...configs[profile],
            exec: "getTournaments",
        },
        // challenges
        getChallenges: {
            ...configs[profile],
            exec: "getChallenges",
        },
        getChallengesCount: {
            ...configs[profile],
            exec: "getChallengesCount",
        },
        // replay elo histories
        getEloHistories: {
            ...configs[profile],
            exec: "getEloHistories",
        },
        // POST testdefs
        // session
        postCreateTempSession: {
            ...configs[profile],
            exec: "postCreateTempSession",
        },
        postRefreshSession: {
            ...configs[profile],
            exec: "postRefreshSession"
        },
        // users
        postUpdateUser: {
            ...configs[profile],
            exec: "postUpdateUser",
        },
        postUploadProfilePic: {
            ...configs[profile],
            exec: "postUploadProfilePic",
        },
        // challenges
        postCreateChallenge: {
            ...configs[profile],
            exec: "postCreateChallenge",
        },
        postUpdateChallenge: {
            ...configs[profile],
            exec: "postUpdateChallenge",
        },
        // games
        postCreateGame: {
            ...configs[profile],
            exec: "postCreateGame",
        }
    },
};

// Test preconditions, setup, and teardown

type SetupData = {
    sessions: Session[];
    gameIds?: string[];
}

export function setup(): SetupData {
    const sessions = setupSessions(usersCount);
    const gameIds = setupGames(gamesCount);
    return { sessions, gameIds };
}

// Additional measurements

export const trendCreateChallenge400 = new Trend("create_challenge_400_duration");
export const trendUpdateChallenge404 = new Trend("update_challenge_404_duration");
export const trendGetTournament404 = new Trend("get_tournament_404_duration");

const endpointNames = [
    "GetLeaderboard", 
    "GetTournaments", 
    "GetTournament", 
    "GetChallenges", 
    "GetChallengesCount",
    "GetReplayByID", 
    "GetReplayMoveList", 
    "SearchReplays", 
    "SearchReplaysWinnerLoserID",
    "SearchReplaysWhiteBlackID", 
    "SearchReplaysTimeframe", 
    "SearchReplaysModeResultCause",
    "GetPlayer", 
    "GetSelfPlayer", 
    "GetPlayerActivity", 
    "GetProfilePic", 
    "SearchPlayers",
    "GetEloHistories",
    "GetGameRooms", 
    "GetGameChats", 
    "GetGameRoomExists",
    "PostCreateTempSession",
    "PostRefreshSession",
    "PostCreateGame", 
    "PostCreateChallenge",
    "PostUpdateChallenge",
    "PostUploadProfilePic",
    "PostUpdateUser",
] as const;

type EndpointName = (typeof endpointNames)[number];

const endpointTrends: Map<EndpointName, Trend> = new Map();
for (const name of endpointNames) {
    // `true` marks this as a time metric so k6 formats values as durations (ms) in the summary.
    endpointTrends.set(name, new Trend(`${name}_duration`, true));
}

function recordDuration(name: EndpointName, resp: { timings: { duration: number } }) {
    endpointTrends.get(name)?.add(resp.timings.duration);
}

// GET test implementations

// leaderboard

export function getLeaderboard() {
    const params = new URLSearchParams({ mode: pickElement(gameModes) });
    const resp = http.get(baseUrl + "/leaderboard?" + params.toString());
    recordDuration("GetLeaderboard", resp);
}

// tournament

export function getTournaments() {
    const params = new URLSearchParams({ userId: randUserID() });
    const resp = http.get(baseUrl + "/tournaments?" + params.toString());
    recordDuration("GetTournaments", resp);
}

export function getTournament() {
    const params = new URLSearchParams({ tournamentKey: staticTournamentKey });

    params.responseCallback = expectedStatuses(200, 404); // 404 could be not found tournament, if the tournament key provided in envvars is not found.

    const resp = http.get(baseUrl + "/tournament?" + params.toString());
    recordDuration("GetTournament", resp);

    recordDuration("PostCreateChallenge", resp);
    if (resp.status === 404) {
        trendGetTournament404.add(resp.timings.duration);
    }
}

// challenges

export function getChallenges(data: SetupData) {
    const searchParams = new URLSearchParams({
        participants: randrange(0, 1) === 0 ? "sent" : "received"
    });
    const headerParams = pickSessionParams(data.sessions);
    const resp = http.get(baseUrl + "/challenges?" + searchParams.toString(), headerParams);
    recordDuration("GetChallenges", resp);
}

export function getChallengesCount(data: SetupData) {
    const headerParams = pickSessionParams(data.sessions);
    const resp = http.get(baseUrl + "/challenges/count", headerParams);
    recordDuration("GetChallengesCount", resp);
}

// replays

export function getReplay_ReplayID() {
    const searchParams = new URLSearchParams({
        id: randReplayID()
    });
    const resp = http.get(baseUrl + "/replay?" + searchParams.toString());
    recordDuration("GetReplayByID", resp);
}

export function getReplayMoveList() {
    const params = new URLSearchParams({ replayId: randReplayID() });
    const resp = http.get(baseUrl + "/replay/move-list?" + params.toString());
    recordDuration("GetReplayMoveList", resp);
}

export function searchReplays() {
    const resp = http.get(baseUrl + "/replays");
    recordDuration("SearchReplays", resp);
}

export function searchReplaysWinnerIDLoserID() {
    const params = new URLSearchParams({ 
        winnerId: randUserID(), 
        loserId: randUserID() 
    });
    const resp = http.get(baseUrl + "/replays?" + params.toString());
    recordDuration("SearchReplaysWinnerLoserID", resp);
}

export function searchReplaysWhiteIDBlackID() {
    const params = new URLSearchParams({
        whiteId: randUserID(), 
        blackId: randUserID() 
    });
    const resp = http.get(baseUrl + "/replays?" + params.toString());
    recordDuration("SearchReplaysWhiteBlackID", resp);
}

export function searchReplaysTimeframe() {
    // TODO: find a way to have the seeded data have stable timeframes
    const params = new URLSearchParams({
        fromDate: randDate(2025),
        toDate: randDate(2026),
    });
    const resp = http.get(baseUrl + "/replays?" + params.toString());
    recordDuration("SearchReplaysTimeframe", resp);
}

export function searchReplaysModeResultCause() {
    const params = new URLSearchParams({ 
        mode: pickElement(gameModes), 
        cause: pickElement(replayCauses), 
        result: pickElement(replayResults),
    });
    const resp = http.get(baseUrl + "/replays?" + params.toString());
    recordDuration("SearchReplaysModeResultCause", resp);
}

// players / users

export function getPlayer() {
    const params = new URLSearchParams({ id: randUserID() });
    const resp = http.get(baseUrl + "/players?" + params.toString());
    recordDuration("GetPlayer", resp);
}

export function getSelfPlayer(data: SetupData) {
    const headerParams = pickSessionParams(data.sessions);
    const resp = http.get(baseUrl + "/players/self", headerParams);
    recordDuration("GetSelfPlayer", resp);
}

// TODO: have some of the players be active so this returns something other than false.
export function getPlayerActivity() {
    const params = new URLSearchParams({ userId: randUserID() });
    const resp = http.get(baseUrl + "/players/activity?" + params.toString());
    recordDuration("GetPlayerActivity", resp);
}

export function getProfilePic() {
    const params = new URLSearchParams({ userId: randUserID() });
    const resp = http.get(baseUrl + "/users/profile-pics?" + params.toString());
    recordDuration("GetProfilePic", resp);
}

export function searchPlayers() {
    const params = new URLSearchParams({ username: "John" });
    const resp = http.get(baseUrl + "/players/search?" + params.toString());
    recordDuration("SearchPlayers", resp);
}

export function getEloHistories() {
    const params = new URLSearchParams({ userId: randUserID() });
    const resp = http.get(baseUrl + "/replay/elo-histories?" + params.toString());
    recordDuration("GetEloHistories", resp);
}

// game / rooms

export function getGameRooms(data: SetupData) {
    const params = pickSessionParams(data.sessions);
    const resp = http.get(baseUrl + "/game/rooms", params);
    recordDuration("GetGameRooms", resp);
}

export function getGameChats(data: SetupData) {
    const params = new URLSearchParams({ gameId: pickElement(data.gameIds) });
    const resp = http.get(baseUrl + "/game/rooms/chats?" + params.toString());
    recordDuration("GetGameChats", resp);
}

export function getGameRoomExists(data: SetupData) {
    const params = new URLSearchParams({ gameId: pickElement(data.gameIds) });
    const resp = http.get(baseUrl + "/game/rooms/exists?" + params.toString());
    recordDuration("GetGameRoomExists", resp);
}

// POST test implementations

// session

export function postCreateTempSession(data: SetupData) {
    const params = pickSessionParams(data.sessions);
    const resp = http.post(baseUrl + "/session/temp", null, params);
    recordDuration("PostCreateTempSession", resp);
}

export function postRefreshSession(data: SetupData) {
    const params = pickSessionParams(data.sessions);
    const resp = http.post(baseUrl + "/session/refresh", null, params);
    recordDuration("PostRefreshSession", resp);
}

// game

export function postCreateGame() {
    const body = JSON.stringify({
        mode: pickElement(gameModes),
        firstColor: pickElement(colors),
    });
    const resp = http.post(baseUrl + "/games/create", body);
    recordDuration("PostCreateGame", resp);
}

// challenges

export function postCreateChallenge(data: SetupData) {
    const body = JSON.stringify({
        challengeeId: randUserIDInt(),
        mode: pickElement(gameModes),
        startColor: pickElement(colors),
    });
    const params = pickSessionParams(data.sessions);

    params.responseCallback = expectedStatuses(200, 400); // 400 could be a duplicate or self challenge, which are not classified as perf test failures

    const resp = http.post(baseUrl + "/challenges/create", body, params);
    recordDuration("PostCreateChallenge", resp);
    if (resp.status === 400) {
        trendCreateChallenge400.add(resp.timings.duration);
    }
}

export function postUpdateChallenge(data: SetupData) {
    const [params, userId] = pickSession(data.sessions);
    const body = JSON.stringify({
        challengerID: randUserIDInt(),
        challengeeID: userId,
        action: "ACCEPT",
    });

    params.responseCallback = expectedStatuses(200, 404); // challenge may not exist, this is expected

    const resp = http.post(baseUrl + "/challenges/update", body, params);

    recordDuration("PostUpdateChallenge", resp);
    if (resp.status === 404) {
        trendUpdateChallenge404.add(resp.timings.duration);
    }
}

// user / player

const inputFile = open("../inputs/sample-image.png", "b");
const inputFileChecksum = "q3ayd2OBXomVIgArjuf4rkuvzYDh2Ju2rxEWq1zCuCM="; // hardcoded, you must change this if you change the input file

export function postUploadProfilePic(data: SetupData) {
    const params = pickSessionParams(data.sessions);

    params.headers["Content-Type"] = "image/png";
    params.headers["Content-Digest"] = inputFileChecksum;
    params.headers["Content-Length"] = inputFile.byteLength;

    const resp = http.post(baseUrl + "/users/profile-pics", inputFile, params);
    recordDuration("PostUploadProfilePic", resp);
}

export function postUpdateUser(data: SetupData) {
    const body = JSON.stringify({ 
        // we're avoiding updating the username as to keep the username preconditions stable for the next perf test execution.
        // bio is the bulk of the data that is updated, and is sufficient to test this endpoint.
        newCountry: "us",
        newBio: faker.word.loremIpsumParagraph(1, 3, 8, "\n"),
    });
    const params = pickSessionParams(data.sessions);

    const resp = http.post(baseUrl + "/users", body, params);
    recordDuration("PostUpdateUser", resp);
}
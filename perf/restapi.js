import http from "6/http";

const hostname = __ENV.HOSTNAME || "http://localhost:8080";
const baseUri = `${hostname}/api`;

const constantArrivalRate = {
    executor: "constant-arrival-rate",
    rate: 20,
    timeUnit: "1s",
    duration: "1m",
    preAllocatedVUs: 10,
    maxVUs: 100,
};

export const options = {
    scenarios: {
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
            exec: "searchReplays_byWinnerID_LoserID",
        },
        search_replays_black_white_id: {
            ...constantArrivalRate,
            exec: "searchReplays_byWhiteID_BlackID",
        },
        search_replays_timeframe: {
            ...constantArrivalRate,
            exec: "searchReplays_byTimeframe",
        },
        search_replays_mode_result_cause: {
            ...constantArrivalRate,
            exec: "searchReplays_byModeResultCause",
        },
        // users
        search_users: {
            ...constantArrivalRate,
            exec: "searchUsers",
        },
        // game metadata
        get_game_metadatas: {
            ...constantArrivalRate,
            exec: "getGameMetadatas",
        },
        // replays
        get_elo_histories: {
            ...constantArrivalRate,
            exec: "getEloHistories",
        },
    },
};

function randint(min, max) {
    return Math.floor(Math.random() * (max - min + 1)) + min;
}

const leaderboardModes = [
    "CORRESPONDENCE_1",
    "CORRESPONDENCE_7",
    "CORRESPONDENCE_14",
    "TIMED_1+0",
    "TIMED_3+2",
    "TIMED_15+10"
];

export function getLeaderboard() {
    const mode = leaderboardModes[randint(0, leaderboardModes.length - 1)];
    const params = new URLSearchParams({ mode });
    http.get(baseUri + "/leaderboard?" + params.toString());
}

export function getTournaments() {
    http.get(baseUri + "/tournaments");
}

export function getTournament() {
    http.get(baseUri + "/tournament");
}

export function getGameMetadatas() {
    http.get(baseUri + "/game/rooms");
}

export function searchReplays() {
    http.get(baseUri + "/replays");
}

export function searchReplays_byWinnerID_LoserID() {
    const params = new URLSearchParams({ winnerID: "1", loserID: "2" });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_byWhiteID_BlackID() {
    const params = new URLSearchParams({ winnerID: "1", loserID: "2" });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_byTimeframe() {
    const params = new URLSearchParams({ winnerID: "1", loserID: "2" });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_byModeResultCause() {
    const params = new URLSearchParams({ mode: "CORRESPONDENCE_7", cause: "WHITE_WINS", result: "CHECKMATE" });
    http.get(baseUri + "/replays?" + params.toString());
}

// these are guaranteed to be contained within the test data.
const searchableUsernames = [
    "John",
    "Tom",
    "Joe",
];

export function searchUsers() {
    const username = leaderboardModes[randint(0, searchableUsernames.length - 1)];
    const params = new URLSearchParams({ username });
    http.get(baseUri + "/players/search?" + params.toString());
}

export function getEloHistories() {
    http.get(baseUri + "/replay/elo-histories");
}
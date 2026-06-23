import http from "k6/http";
import { URLSearchParams } from "https://jslib.k6.io/url/1.0.0/index.js";

// Environment variables and shell arguments

const hostname = __ENV.HOSTNAME || "http://localhost:8081";
const baseUri = `${hostname}/api`;

const constantArrivalRate = {
    executor: "constant-arrival-rate",
    rate: 20,
    timeUnit: "1s",
    duration: "1m",
    preAllocatedVUs: 10,
    maxVUs: 100,
};

// VU and perf test configuration

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

const minUser = 1;
const maxUser = 1000;

// seed script will generate a sequence of continously increasing uuids, starting from 0, so we can safely randrange a tournament key
const minTournamentKey = 1;
const maxTournamentKey = 250;

function randrange(min, max) {
    return Math.floor(Math.random() * (max - min + 1)) + min; // inclusive range (min, max)
}

function randUserID() {
    return String(randrange(minUser, maxUser));
}

const leaderboardModes = [
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

function randEnum(enumArray) {
    return enumArray[randrange(0, enumArray.length - 1)]
}

// Test implementations

export function getLeaderboard() {
    const params = new URLSearchParams({ mode: randEnum(leaderboardModes) });
    http.get(baseUri + "/leaderboard?" + params.toString());
}

export function getTournaments() {
    http.get(baseUri + "/tournaments");
}

export function getTournament() {
    const params = new URLSearchParams({ tournamentKey: randrange(minTournamentKey, maxTournamentKey) });
    http.get(baseUri + "/tournament");
}

export function getGameMetadatas() {
    http.get(baseUri + "/game/rooms");
}

export function searchReplays() {
    http.get(baseUri + "/replays");
}

export function searchReplays_byWinnerID_LoserID() {
    const params = new URLSearchParams({ winnerId: randUserID(), loserId: randUserID() });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_byWhiteID_BlackID() {
    const params = new URLSearchParams({ whiteId: randUserID(), blackId: randUserID() });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_byTimeframe() {
   const params = new URLSearchParams({ winnerId: randUserID(), loserId: randUserID() });
    http.get(baseUri + "/replays?" + params.toString());
}

export function searchReplays_byModeResultCause() {
    const params = new URLSearchParams({ 
        mode: randEnum(leaderboardModes), 
        cause: randEnum(replayCauses), 
        result: randEnum(replayResults),
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
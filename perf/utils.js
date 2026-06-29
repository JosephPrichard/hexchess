export const gameModes = [
    "CORRESPONDENCE_1",
    "CORRESPONDENCE_7",
    "CORRESPONDENCE_14",
    "TIMED_1+0",
    "TIMED_3+2",
    "TIMED_15+10"
];

export const replayCauses = [
    "CHECKMATE",
    "STALEMATE",
    "FORFEIT"
];

export const replayResults = [
    "WHITE_WINS",
    "BLACK_WINS",
    "DRAW",
];

export const colors = [
    "WHITE",
    "BLACK",
    "RANDOM"
];

export function pickElement(array) {
    if (array.length == 0) {
        throw new Error("cannot pick an element on an empty array");
    }
    return array[randrange(0, array.length - 1)];
}

export function makeSessionParams(sessions) {
    const [params, _] = pickSession(sessions);
    return params;
}

export function pickSession(sessions) {
    const session = pickElement(sessions);
    const cookie = session.cookie;
    const params = { 
        headers: { "Cookie": cookie }
    };
    return [params, session.userId];
}

export function randrange(min, max) {
    return Math.floor(Math.random() * (max - min + 1)) + min; // inclusive range (min, max)
}

export function randUserID() {
    return String(randUserIDInt());
}

export function randUserIDInt() {
    return randrange(minUserID, maxUserID);
}

export function randTournamentKey() {
    const tkeyint = randrange(minTournamentKey, maxTournamentKey);
    return uuidFromBigInt(tkeyint);
}

export function randReplayID() {
    return randrange(minReplayID, maxReplayID);
}

export function randDate(year) {
    const days = String(randrange(1, 28)).padStart(2, '0');
    const month = String(randrange(1, 12)).padStart(2, '0');
    return `${String(year)}-${month}-${days}`;
}

const minUserID = 1;
const maxUserID = 1000;

// seed script will generate a sequence of continously increasing uuids, starting from 0, so we can safely randrange a tournament key
const minTournamentKey = 1;
const maxTournamentKey = 250;

const minReplayID = 1;
const maxReplayID = 5000;

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
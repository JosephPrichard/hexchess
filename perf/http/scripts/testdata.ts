import { Session } from "./setup.ts";

const minUserID = 1;
const maxUserID = parseInt(__ENV.MAX_USER_ID) || 1000;

const minReplayID = 1;
const maxReplayID = parseInt(__ENV.MAX_REPLAY_ID) || 5000;

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

export function pickElement<T>(array?: T[]) {
    if (!array || array.length == 0) {
        throw new Error("cannot pick an element on an empty array");
    }
    return array[randrange(0, array.length - 1)];
}

export function pickNSessions(sessions: Session[], n: number) {
    if (sessions.length < n) {
        throw new Error(`Cannot pick ${n} sessions from pool of ${sessions.length}`)
    }

    const randomSessions: Session[] = [];
    const pickedSessions: Record<number, boolean> = {};

    for (let i = 0; i < n; i++) {
        while (true) {
            const sessionIdx = randrange(0, sessions.length - 1);
            if (!pickedSessions[sessionIdx]) {
                randomSessions.push(sessions[sessionIdx]);
                pickedSessions[sessionIdx] = true;
                break;
            }
        }
    }

    return randomSessions;
}

export function makeSessionParams(session: Session): any {
    const cookie = session.cookie;
    return { 
        headers: { "Cookie": cookie }
    };
}

export function pickSessionParams(sessions: Session[]) {
    const [params, _] = pickSession(sessions);
    return params;
}

export function pickSession(sessions: Session[]): [any, number | undefined] {
    const session = pickElement(sessions);
    const params = makeSessionParams(session);
    return [params, session.userId];
}

export function randrange(min: number, max: number) {
    return Math.floor(Math.random() * (max - min + 1)) + min; // inclusive range (min, max)
}

export function randUserID() {
    return String(randUserIDInt());
}

export function randUserIDInt() {
    return randrange(minUserID, maxUserID);
}

export function randReplayID() {
    return randrange(minReplayID, maxReplayID);
}

export function randDate(year: number) {
    const days = String(randrange(1, 28)).padStart(2, '0');
    const month = String(randrange(1, 12)).padStart(2, '0');
    return `${String(year)}-${month}-${days}`;
}
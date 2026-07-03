import http from "k6/http";
import { EventName, WebSocket, MessageEvent, ErrorEvent, BinaryType } from "k6/websockets";
import { Session, setupSessions } from "./setup.ts"
import { gameModes, pickElement, makeSessionParams, pickNSessions } from "./testdata.ts"
import { TrendCounter } from "./trends.ts";

// ignore typechecking for CDN imports and K6 extensions
// @ts-ignore
import { runGameSockets } from "k6/x/hexchess/websocket";

const restProtocol = __ENV.PROTOCOL ?? "http";
const wsProtocol = __ENV.PROTOCOL ?? "ws";
const hostname = __ENV.HOSTNAME ?? "localhost:8081";

export const restBaseUrl = `${restProtocol}://${hostname}/api`;
export const wsBaseUrl = `${wsProtocol}://${hostname}`;

const usersCount = parseInt(__ENV.USERS_COUNT) || 10;
const playersCount = parseInt(__ENV.PLAYERS_COUNT) || 2;
const maxMoves = parseInt(__ENV.MAX_MOVES) || 50;

let timeoutSecs: number = 0;
let staggerMs: number = 0;

if (usersCount < playersCount) {
    throw new Error("total users count must be at least number of players per game");
}
if (playersCount < 2) {
    throw new Error("players per game count should be at least 2");
}

const profile = __ENV.PROFILE || "smoke";

const gamesocketScenario = function() {
    if (profile === "smoke") {
        staggerMs = 0;
        return {
            executor: "shared-iterations",
            vus: 1,
            iterations: 1,
            maxDuration: "60s",
        };
    } else if (profile === "capacity") {
        staggerMs = 250;
        return {
            executor: "constant-vus",
            vus: 50,
            duration: "5m",
        };
    } else if (profile === "idle") {
        const staggerSecs = 10;
        staggerMs = 1000 * staggerSecs;
        timeoutSecs = staggerSecs * (maxMoves + 1);
        return {
            executor: "constant-vus",
            vus: 1000,
            duration: "5m",
        };
    }
}();

export const options = {
    scenarios: { gamesocket: gamesocketScenario },
}

type SetupData = {
    sessions: Session[];
}

export function setup(): SetupData {
    const sessions = setupSessions(usersCount);
    return { sessions };
}

const trendKey = (i: string, o: string) => `${i}->${o}`;

export const trendMoveIntoMove = new TrendCounter("move_messages");
export const trendJoinIntoInit = new TrendCounter("init_messages");
export const trendMoveIntoErrors = new TrendCounter("invalid_move_messages");
export const trendChatIntoChat = new TrendCounter("chat_messages");

const messageTrends = {
    [trendKey("move", "move")]: trendMoveIntoMove,
    [trendKey("open", "init")]: trendJoinIntoInit,
    [trendKey("move", "error")]: trendMoveIntoErrors,
    [trendKey("chat", "chat")]: trendChatIntoChat,
};

export default function (data: SetupData) {
    // setup
    const body = JSON.stringify({
        mode: pickElement(gameModes),
        firstColor: "WHITE",
    });
    let resp = http.post(restBaseUrl + "/games/create", body);
    if (resp.status != 200) {
        throw new Error("!2xx response while creating game");
    }
    const gameId = JSON.parse(resp.body as string).gameId;

    const tempSessionIds: string[] = [];

    for (const session of pickNSessions(data.sessions, 2)) {
        resp = http.post(restBaseUrl + "/session/temp", null, makeSessionParams(session));
        if (resp.status != 200) {
            throw new Error("!2xx response while creating sessions");
        }
        tempSessionIds.push(String(JSON.parse(resp.body as string).sessionId));
    }

    // execute
    const { metrics, error } = runGameSockets({
        targetEndpoint: wsBaseUrl,
        gameId: gameId,
        sessionIds: tempSessionIds,
        staggerMs: staggerMs,
        timeoutSecs: timeoutSecs,
        maxMoves: maxMoves
    });
    if (error) {
        throw new Error("failed to run gamesockets: " + error);
    }

    // measure
    for (const [_, metric] of Object.entries(metrics)) {
        for (const outputMetric of metric.outputs) {
            const key = trendKey(metric.inputType, outputMetric.outputType);
            const trend = messageTrends[key];
            if (!trend) {
                throw new Error("unknown trend for key: " + key);
            }
            trend.add(outputMetric.outputTime - metric.inputTime);
        }
    }
}
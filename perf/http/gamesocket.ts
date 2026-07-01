import http from "k6/http";
import { Trend, Counter } from "k6/metrics";
import { EventName, WebSocket, MessageEvent, ErrorEvent, BinaryType } from "k6/websockets";
import { Session, setupSessions } from "./setup.ts"
import { gameModes, colors, pickElement, makeSessionParams } from "./utils.ts"
import { MetricMap } from "./metrics.ts";

// ignore typechecking for CDN imports and K6 extensions
// @ts-ignore
import { URLSearchParams } from "https://jslib.k6.io/url/1.0.0/index.js";
// @ts-ignore
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
// @ts-ignore
import { handleGameInputEvent, createChatGameInput } from "k6/x/hexchess/websocket";

const restProtocol = __ENV.PROTOCOL ?? "http";
const wsProtocol = __ENV.PROTOCOL ?? "ws";
const hostname = __ENV.HOSTNAME ?? "localhost:8081";

export const restBaseUrl = `${restProtocol}://${hostname}/api`;
export const wsBaseUrl = `${wsProtocol}://${hostname}/api`;

const usersCount = parseInt(__ENV.USERS_COUNT) || 10;
const playersCount = parseInt(__ENV.PLAYERS_COUNT) || 2;
const maxMessageCount = parseInt(__ENV.MAX_MESSAGE_COUNT) || Number.MAX_VALUE;
const chatPeriod = parseInt(__ENV.SEND_CHAT_PERIOD_MS) || 1;

if (playersCount < 2) {
    throw new Error("players per game count should be at least 2");
}

// export const options = {
//     executor: "shared-iterations",
//     vus: 1,
//     iterations: 1,
//     maxDuration: "5s",
// };

export const options = {
    executor: "constant-vus",
    vus: 1,
    duration: "1m",
};

type SetupData = {
    sessions: Session[];
}

export function setup(): SetupData {
    const sessions = setupSessions(usersCount);
    return { sessions };
}

const trendKey = (i: string, o: string) => `${i}->${o}`;

class CustomTrend {
    trend: Trend;
    count: Counter;

    constructor(name: string) {
        this.trend = new Trend(name, true);
        this.count = new Counter(`${name}_count`);
    }

    add(value: number | boolean, tags?: {[name: string]: string}) {
        this.trend.add(value, tags);
        this.count.add(1, tags);
    }
}

export const trendMoveIntoMove = new CustomTrend("move_messages");
export const trendJoinIntoInit = new CustomTrend("init_messages");
export const trendMoveIntoErrors = new CustomTrend("invalid_move_messages");
export const trendChatIntoChat = new CustomTrend("chat_messages");

const messageTrends = {
    [trendKey("move", "move")]: trendMoveIntoMove,
    [trendKey("open", "init")]: trendJoinIntoInit,
    [trendKey("move", "error")]: trendMoveIntoErrors,
    [trendKey("chat", "chat")]: trendChatIntoChat,
};

export default async function (data: SetupData) {
    const metrics = new MetricMap();

    // setup: create a game and users in
    const createBody = JSON.stringify({
        mode: pickElement(gameModes),
        firstColor: "WHITE",
    });
    const createResp = http.post(restBaseUrl + "/games/create", createBody);
    const gameId = JSON.parse(createResp.body as string).gameId;

    const sessionIds: string[] = [];
    for (let i = 0; i < playersCount; i++) {
        const sessParams = makeSessionParams(data.sessions);
        const sessResp = http.post(restBaseUrl + "/session/temp", null, sessParams);
        sessionIds.push(String(JSON.parse(sessResp.body as string).sessionId));
    }

    // excute: play by responding to output from server until game is terminal
    const gamePromises = sessionIds.map((sessionId) => connectGame(metrics, gameId, sessionId));
    await Promise.all(gamePromises);
    
    // measure: compute metrics that cannot be calculated automatically by grafana such as end to end message transmission time
    metrics.iterateMetrics((inputMetric, outputMetric) => {
        const key = trendKey(inputMetric.inputType, outputMetric.outputType);
        const trend = messageTrends[key];
        if (!trend) {
            console.error("unknown trend for key", key);
            return;
        }
        trend.add(outputMetric.outputTime.getTime() - inputMetric.inputTime.getTime());
    });
}

const connectGame = (metrics: MetricMap, gameId: string, sessionId: string) => new Promise<void>((resolveWebsocket) => {
    let messageCount = 0;

    let chatInterval: number | undefined;

    const params = new URLSearchParams({ gameId, sessionId })
    const url = `${wsBaseUrl}/ws/game?${params}`;

    const openMessageId = uuidv4();
    metrics.newMetric(openMessageId, "open");

    const socket = new WebSocket(url);
    socket.binaryType = "arraybuffer" as BinaryType;

    socket.onopen = () => {
        console.log(`gameId=${gameId} open`);
    }

    socket.onclose = () => {
        console.log(`gameId=${gameId} close`);
        resolveWebsocket();
        if (chatInterval) {
            clearInterval(chatInterval);
        }
    };

    socket.onmessage = (event?: MessageEvent | ErrorEvent) => {
        const recvBytes = (event as MessageEvent).data;
        if (!(recvBytes instanceof ArrayBuffer)) {
            console.error(`gameId=${gameId} received non-binary message`);
            return;
        }
        const inputMessageId = uuidv4();

        const {
            // extracted from recvBytes
            prevMessageId,
            prevType,
            isTerminal,
            // produced for next send call
            nextInputBytes
        } = handleGameInputEvent({ recvBytes, inputMessageId });

        if (nextInputBytes) {
            socket.send(nextInputBytes);
            metrics.newMetric(inputMessageId, "move");
        }
        if (prevType === "init" && !prevMessageId) {
            metrics.appendMetric(openMessageId, prevType);
        }
        if (prevMessageId) {
            metrics.appendMetric(prevMessageId, prevType);
        }
        if (isTerminal || messageCount >= maxMessageCount) {
            socket.close();
        }
        messageCount++;
    };
})
import { check } from "k6";
import http, { expectedStatuses } from "k6/http";
import { Trend, Counter } from "k6/metrics";
import { WebSocket } from "k6/websockets";
import { URLSearchParams } from "https://jslib.k6.io/url/1.0.0/index.js";
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { handleGameOutputEvent } from "k6/x/hexchess/websocket";
import { setupSessions } from "./setup.js"
import { gameModes, colors, pickElement, makeSessionParams } from "./utils.js"

const restProtocol = __ENV.PROTOCOL || "http";
const wsProtocol = __ENV.PROTOCOL || "ws";
const hostname = __ENV.HOSTNAME || "localhost:8081";

export const restBaseUrl = `${restProtocol}://${hostname}/api`;
export const wsBaseUrl = `${wsProtocol}://${hostname}/api`;

const usersCount = parseInt(__ENV.USERS_COUNT) || 10;
const playersCount = parseInt(__ENV.PLAYERS_COUNT) || 2;
const maxMessageCount = parseInt(__ENV.MAX_MESSAGE_COUNT) || Number.MAX_VALUE;

if (playersCount < 2) {
    throw new Error("players per game count should be at least 2");
}

export const options = {
    fixed_iterations: {
        executor: "shared-iterations",
        vus: 1,
        iterations: 1,
        maxDuration: "5s",
    }
};

export function setup() {
    const sessions = setupSessions(usersCount);
    return { sessions };
}

const makeTrendKey = (i, o) => `${i}->${o}`;

function newTrend(name) {
    return {
        trend: new Trend(name, true),
        count: new Counter(`${name}_count`),
        add(val, tags) {
            this.trend.add(val, tags);
            this.count.add(1, tags);
        }
  };
}

export const trendMoveIntoMove = newTrend("move_messages");
export const trendJoinIntoInit = newTrend("init_messages");
export const trendMoveIntoErrors = newTrend("invalid_move_messages");

const messageTrends = {
    [makeTrendKey("move", "move")]: trendMoveIntoMove,
    [makeTrendKey("open", "init")]: trendJoinIntoInit,
    [makeTrendKey("move", "error")]: trendMoveIntoErrors,
};

export default async function (data) {
    const messageMetrics = new Map();

    // setup: create a game and users in
    const createBody = JSON.stringify({
        mode: pickElement(gameModes),
        firstColor: pickElement(colors),
    });
    const createResp = http.post(restBaseUrl + "/games/create", createBody);
    const gameId = JSON.parse(createResp.body).gameId;

    const sessionIds = [];
    for (let i = 0; i < playersCount; i++) {
        const sessParams = makeSessionParams(data.sessions);
        const sessResp = http.post(restBaseUrl + "/session/temp", null, sessParams);
        sessionIds.push(JSON.parse(sessResp.body).sessionId)
    }

    // excute: play by responding to output from server until game is terminal
    const gameConnections = sessionIds.map((sessionId) => connectGame(messageMetrics, gameId, sessionId));
    await Promise.all(gameConnections);
    
    // measure: compute metrics that cannot be calculated automatically by grafana such as end to end message transmission time
    for (const [_, metric] of messageMetrics) {
        for (const outputMetric of metric.outputs) {
            const trendKey = makeTrendKey(metric.inputType, outputMetric.outputType);
            const trend = messageTrends[trendKey];
            if (!trend) {
                console.error("unknown trend", trendKey);
                continue;
            }
            trend.add(outputMetric.outputTime.getTime() - metric.inputTime.getTime());
        }
    }
}

const connectGame = (messageMetrics, gameId, sessionId) => {
    let messageCount = 0;

    // const openInputMessageId = uuidv4();

    return new Promise((resolve) => {
        // messageMetrics[openInputMessageId] = {inputTime: new Date(), inputType: "open", outputs: []};

        const params = new URLSearchParams({ gameId, sessionId })
        const url = `${wsBaseUrl}/ws/game?${params}`;

        const socket = new WebSocket(url);
        socket.binaryType = "arraybuffer";

        socket.addEventListener("error", (e) => console.error(`sessionId=${sessionId} error:`, e));

        socket.addEventListener("close", () => {
            console.log(`sessionId=${sessionId} close`);
            resolve();
        });

        socket.addEventListener("message", (event) => {
            const outputBytes = event.data;
            const inputMessageId = uuidv4();

            const {
                nextInputBytes,
                inputType,
                prevOutputMessageId: outputMessageId,
                prevOutputType: outputType,
                isTerminal
            } = handleGameOutputEvent({
                prevOutputBytes: outputBytes,
                nextInputMessageId: inputMessageId,
            });

            if (outputMessageId) {
                const metric = messageMetrics.get(outputMessageId);
                if (!metric) {
                    throw new Error(`unknown metric for output message id ${outputMessageId}`);
                }
                metric.outputs.push({outputTime: new Date(), outputType: outputType});
            }
            if (nextInputBytes) {
                socket.send(nextInputBytes);
                messageMetrics.set(inputMessageId, {inputTime: new Date(), inputType: inputType, outputs: []});
            }
            if (isTerminal || messageCount >= maxMessageCount) {
                socket.close();
            }
            messageCount++;
        });
    });
}
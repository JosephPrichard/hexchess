import http from "k6/http";
import { gameModes, colors, pickElement } from "./utils.js"

const protocol = __ENV.PROTOCOL || "http";
const hostname = __ENV.HOSTNAME || "localhost:8081";
export const baseUrl = `${protocol}://${hostname}/api`;

export function setupSessions(usersCount) {
    const sessionRequests = [];
    for (let i = 0; i < usersCount; i++) {
        // precondition: seeded data is assumed to have usernames conforming to this pattern, each with the provided password
        const body = JSON.stringify({ username: `User${i}`, password: "password1" });
        sessionRequests.push(["POST", baseUrl + "/login", body]);
    }
    const sessionResps = http.batch(sessionRequests);
    return sessionResps.map(resp => ({ userId: JSON.parse(resp.body).id, cookie: resp.headers["Set-Cookie"] }));
}

export function setupGames(gamesCount) {
    const gameRequests = [];
    for (let i = 0; i < gamesCount; i++) {
        const body = JSON.stringify({ mode: pickElement(gameModes), firstColor: pickElement(colors) });
        gameRequests.push(["POST", baseUrl + "/games/create", body]);
    }
    const gameResps = http.batch(gameRequests);
    return gameResps.map(resp => JSON.parse(resp.body).gameId);
}
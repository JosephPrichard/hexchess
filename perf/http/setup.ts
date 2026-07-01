import http, { BatchRequests } from "k6/http";
import { gameModes, colors, pickElement } from "./utils.ts";

const protocol = __ENV.PROTOCOL || "http";
const hostname = __ENV.HOSTNAME || "localhost:8081";
export const baseUrl = `${protocol}://${hostname}/api`;

export type Session = {
    userId?: number;
    cookie?: string;
}

export function setupSessions(usersCount: number): Session[] {
    const sessionRequests: BatchRequests = [];
    for (let i = 0; i < usersCount; i++) {
        // precondition: seeded data is assumed to have usernames conforming to this pattern, each with the provided password
        const body = JSON.stringify({ 
            username: `User${i}`, 
            password: "password1",
        });
        sessionRequests.push(["POST", baseUrl + "/login", body]);
    }
    const sessionResps = http.batch(sessionRequests);

    return sessionResps.map(resp => ({ 
        userId: Number(JSON.parse(resp.body as string).id), 
        cookie: String(resp.headers["Set-Cookie"]) 
    }));
}

export function setupGames(gamesCount: number): string[] {
    const gameRequests: BatchRequests = [];
    for (let i = 0; i < gamesCount; i++) {
        const body = JSON.stringify({ 
            mode: pickElement(gameModes), 
            firstColor: pickElement(colors),
        });
        gameRequests.push(["POST", baseUrl + "/games/create", body]);
    }
    const gameResps = http.batch(gameRequests);

    return gameResps.map(resp => String(JSON.parse(resp.body as string).gameId));
}
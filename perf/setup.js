import http from "k6/http";
import { gameModes, colors, pickElement } from "./utils.js"

const protocol = __ENV.PROTOCOL || "http";
const hostname = __ENV.HOSTNAME || "localhost:8081";
export const baseUrl = `${protocol}://${hostname}/api`;

export function setupSessions(usersCount) {
     const sessionRequests = [];
    // we need to seed sessions for authenticated requests 
    for (let i = 0; i < usersCount; i++) {
        // precondition: seeded data is assumed to have usernames conforming to this pattern, each with the provided password
        const body = JSON.stringify({ username: `User${i}`, password: "password1" });
        sessionRequests.push(["POST", baseUrl + "/login", body]);
    }
    
    const sessionResps = http.batch(sessionRequests);
    // these session cookies can be used anytime we need an authenticated request.
    // a big assumption is: these stay valid until the end of the test
    return sessionResps.map(resp => ({ userId: JSON.parse(resp.body).id, cookie: resp.headers["Set-Cookie"] }));
}

export function setupGames(gamesCount) {
    const gameRequests = [];
    // create some games beforehand (seperate from the ones created as part of resting the /game/create endpoint) for game data reads
    for (let i = 0; i < gamesCount; i++) {
        const body = JSON.stringify({ mode: pickElement(gameModes), firstColor: pickElement(colors) });
        gameRequests.push(["POST", baseUrl + "/games/create", body]);
    }

    const gameResps = http.batch(gameRequests);

    return gameResps.map(resp => JSON.parse(resp.body).gameId);
}
import type { ChallengeView, ChessBoard, ReplayView, UserView, UserWithReplaysView } from '$lib/models';
import { codes } from '$lib/response';

export interface ApiResult<T> {
    ok: boolean;
    status: number;
    resp: T | undefined;
    err: string;
}

const baseURL = 'http://localhost:8081';

class ApiError extends Error {
    public readonly status: number;

    constructor(message: string, status?: number) {
        super(message);
        this.status = status || 0;
    }
}

export async function api<T>(call: Promise<Response>): Promise<T> {
    let error: ApiError | undefined;
    try {
        const response = await call;

        if (!response.ok) {
            const errorCode = await response.text();
            error = new ApiError(errorCode, response.status);
        } else {
            return await response.json();
        }
    } catch (error) {
        console.error(error);
        throw new ApiError(codes.errorUnknown, 500);
    }

    throw error;
}

export async function unwrap<T>(promiseResp: Promise<T>): Promise<ApiResult<T>> {
    try {
        const resp = await promiseResp;
        return { ok: true, status: 200, resp, err: '' };
    } catch (error) {
        if (error instanceof ApiError) {
            return { ok: false, status: error.status, resp: undefined, err: error.message };
        }
        return { ok: false, status: 500, resp: undefined, err: codes.errorUnknown };
    }
}

export function postLogin(username: string, password: string) {
    return api<string>(
        fetch(`${baseURL}/forms/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ username, password })
        })
    );
}

export function postRegister(username: string, password: string, confirmPassword: string) {
    return api<string>(
        fetch(`${baseURL}/forms/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ username, password, confirmPassword })
        })
    );
}

export function postUpdateUser(username: string, bio: string, country: string) {
    return api<string>(
        fetch(`${baseURL}/forms/users`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ newUsername: username, newBio: bio, newCountry: country })
        })
    );
}

export function postUpdatePassword(password: string, newPassword: string, confirmNewPassword: string) {
    return api<string>(
        fetch(`${baseURL}/forms/users/password`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ password, newPassword, confirmNewPassword })
        })
    );
}

export function postCreateChallenge(timeControl: string, startColor: string) {
    return api<string>(
        fetch(`${baseURL}/forms/challenges/create`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ startColor, timeControl })
        })
    );
}

export interface UpdateChallengeResp {
    code: string;
    gameId?: string;
}

export function postUpdateChallenge(challengerId: number, challengeeId: number, action: string) {
    return api<UpdateChallengeResp>(
        fetch(`${baseURL}/forms/challenges/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ challengerId, challengeeId, action })
        })
    );
}

export interface UpdateGameResp {
    code: string;
    gameId?: string;
}

export function postCreateGame(timeControl: string, firstColor: string) {
    return api<UpdateGameResp>(
        fetch(`${baseURL}/forms/game/create`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ firstColor, timeControl })
        })
    );
}

export function postLogout() {
    return api<string>(
        fetch(`${baseURL}/forms/logout`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include'
        })
    );
}

export function getReplays(userId: number, afterId?: number) {
    const params = new URLSearchParams({ userId: userId.toString() });
    if (afterId) {
        params.set('afterId', afterId.toString());
    }

    return api<ReplayView[]>(fetch(`${baseURL}/views/replays?${params.toString()}`, { method: 'GET' }));
}

export function getChallenges(participants: string) {
    const params = new URLSearchParams({ participant: participants });

    return api<ChallengeView[]>(
        fetch(`${baseURL}/views/player/challenges?${params.toString()}`, {
            method: 'GET',
            credentials: 'include'
        })
    );
}

export interface LeaderboardResp {
    totalPages: number;
    userList: UserView[];
}

export function getLeaderboard(page: number) {
    const params = new URLSearchParams({ page: String(page) });

    return api<LeaderboardResp>(fetch(`${baseURL}/views/leaderboard?${params}`, { method: 'GET' }));
}

export function getProfile() {
    return api<UserView>(
        fetch(`${baseURL}/views/players/self`, {
            method: 'GET',
            credentials: 'include'
        })
    );
}

export function getUserWithReplays(id: string) {
    return api<UserWithReplaysView>(fetch(`${baseURL}/views/players/${id}`, { method: 'GET' }));
}

export function getSearchPlayers(username: string, page?: number) {
    const params = new URLSearchParams({ username });
    if (page) {
        params.set('page', String(page));
    }

    return api<UserView[]>(fetch(`${baseURL}/views/players/search?${params.toString()}`, { method: 'GET' }));
}

export function getReplay(id: string) {
    return api<ReplayView>(fetch(`${baseURL}/views/replay/${id}`, { method: 'GET' }));
}

export function getCountries() {
    return api<string[]>(fetch(`${baseURL}/views/countries`, { method: 'GET' }));
}

let cachedBoard: ChessBoard | undefined = undefined;

export async function getInitialBoard() {
    if (cachedBoard !== undefined) {
        return cachedBoard;
    }
    const board = await api<ChessBoard>(fetch(`${baseURL}/views/initial-board`, { method: 'GET' }));
    if (!board) {
        return undefined;
    }
    cachedBoard = board;
    return cachedBoard;
}
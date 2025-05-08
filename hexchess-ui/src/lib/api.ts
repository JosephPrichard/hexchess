import type { ChallengeView, ReplayView, UserView, UserWithReplaysView } from '$lib/models';

export interface ApiResult<T> {
    ok: boolean;
    status: number;
    err: string;
    resp: T | undefined;
}

const baseURL = "http://localhost:8081";

async function handleResponse<T>(response: Response): Promise<ApiResult<T>> {
    if (!response.ok) {
        const errorText = await response.text();
        return { ok: false, status: response.status, err: errorText, resp: undefined };
    }
    const data = await response.json();
    return { ok: true, status: response.status, err: "", resp: data };
}

const API_RESULT_UNKNOWN: ApiResult<never> = { ok: false, status: 0, err: 'ERROR_UNKNOWN', resp: undefined };

export async function postLogin(username: string, password: string): Promise<ApiResult<string>> {
    try {
        const response = await fetch(`${baseURL}/forms/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: "include",
            body: JSON.stringify({ username, password })
        });

        return handleResponse<string>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function postRegister(username: string, password: string, confirmPassword: string): Promise<ApiResult<string>> {
    try {
        const response = await fetch(`${baseURL}/forms/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: "include",
            body: JSON.stringify({ username, password, confirmPassword })
        });

        return handleResponse<string>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function postUpdateUser(username: string, bio: string, country: string): Promise<ApiResult<string>> {
    try {
        const response = await fetch(`${baseURL}/forms/users`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: "include",
            body: JSON.stringify({
                newUsername: username,
                newBio: bio,
                newCountry: country
            })
        });

        return handleResponse<string>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function postUpdatePassword(password: string, newPassword: string, confirmNewPassword: string): Promise<ApiResult<string>> {
    try {
        const response = await fetch(`${baseURL}/forms/users/password`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: "include",
            body: JSON.stringify({ password, newPassword, confirmNewPassword })
        });

        return handleResponse<string>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function postCreateChallenge(timeControl: string, startColor: string): Promise<ApiResult<string>> {
    try {
        const response = await fetch(`${baseURL}/forms/challenges/create`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: "include",
            body: JSON.stringify({ startColor, timeControl })
        });

        return handleResponse<string>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export interface UpdateChallengeResp {
    code: string;
    gameId?: string;
}

export async function postUpdateChallenge(challengerId: number, challengeeId: number, action: string): Promise<ApiResult<UpdateChallengeResp>> {
    try {
        const response = await fetch(`${baseURL}/forms/challenges/update`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: "include",
            body: JSON.stringify({ challengerId, challengeeId, action })
        });

        return handleResponse<UpdateChallengeResp>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export interface UpdateGameResp {
    code: string;
    gameId?: string;
}

export async function postCreateGame(timeControl: string, firstColor: string): Promise<ApiResult<UpdateGameResp>> {
    try {
        const response = await fetch(`${baseURL}/forms/game/create`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: "include",
            body: JSON.stringify({ firstColor, timeControl })
        });

        return handleResponse<UpdateGameResp>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function postLogout(): Promise<ApiResult<string>> {
    try {
        const response = await fetch(`${baseURL}/forms/logout`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: "include"
        });

        return handleResponse<string>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function getReplays(userId: number, afterId?: number): Promise<ApiResult<ReplayView[]>> {
    try {
        const params = new URLSearchParams({ userId: userId.toString() });
        if (afterId) {
            params.set('afterId', afterId.toString());
        }

        const response = await fetch(`${baseURL}/views/replays?${params.toString()}`, { method: 'GET' });

        return handleResponse<ReplayView[]>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function getChallenges(participants: string): Promise<ApiResult<ChallengeView[]>> {
    try {
        const params = new URLSearchParams({ participant: participants });

        const response = await fetch(`${baseURL}/views/player/challenges?${params.toString()}`, {
            method: 'GET',
            credentials: "include",
        });

        return handleResponse<ChallengeView[]>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export interface LeaderboardResp {
    totalPages: number;
    userList: UserView[];
}

export async function getLeaderboard(page: number): Promise<ApiResult<LeaderboardResp>> {
    try {
        const params = new URLSearchParams({ page: String(page) });

        const response = await fetch(`${baseURL}/views/leaderboard?${params}`, { method: 'GET' });

        return handleResponse<LeaderboardResp>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function getSelfUser(): Promise<ApiResult<UserView>> {
    try {
        const response = await fetch(`${baseURL}/views/players/self`, {
            method: 'GET',
            credentials: "include",
        });

        return handleResponse<UserView>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function getUserWithReplays(id: string): Promise<ApiResult<UserWithReplaysView>> {
    try {
        const response = await fetch(`${baseURL}/views/players/${id}`, { method: 'GET' });

        return handleResponse<UserWithReplaysView>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function getSearchPlayers(username: string, page?: number): Promise<ApiResult<UserView[]>> {
    try {
        const params = new URLSearchParams({ username });
        if (page) {
            params.set('page', String(page));
        }

        const response = await fetch(`${baseURL}/views/players/search?${params.toString()}`, { method: 'GET' });

        return handleResponse<UserView[]>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function getReplay(username: string): Promise<ApiResult<ReplayView>> {
    try {
        const response = await fetch(`${baseURL}/views/replay/${username}`, { method: 'GET' });

        return handleResponse<ReplayView>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}

export async function getCountries(): Promise<ApiResult<string[]>> {
    try {
        const response = await fetch(`${baseURL}/views/countries`, { method: 'GET' });

        return handleResponse<string[]>(response);
    } catch (error) {
        console.error(error);
        return API_RESULT_UNKNOWN;
    }
}
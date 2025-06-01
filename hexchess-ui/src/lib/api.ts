import type { Challenge, Replay, User, UserWithReplays, Session, Action } from '$lib/models';
import { codes } from '$lib/error';
import type { ChessBoard, PieceMove } from '$lib/messages';

export interface ApiResult<T> {
	ok: boolean;
	status: number;
	resp: T | undefined;
	err: string;
}

export const appBaseURL = 'http://localhost:5173';
export const baseURL = 'http://localhost:8081';

class ApiError extends Error {
	public readonly status: number;

	constructor(message: string, status?: number) {
		super(message);
		this.status = status || 0;
	}
}

export async function api<T>(input: RequestInfo | URL, init?: RequestInit, request?: typeof window.fetch): Promise<T> {
	if (!request) {
		request = fetch;
	}

	let error: ApiError | undefined;
	try {
		const response = await request(input, init);

		if (!response.ok) {
			const errorCode = await response.text();
			error = new ApiError(errorCode, response.status);
		} else {
			if ((response.headers.get('Content-Type') || '').includes('application/json')) {
				return await response.json();
			} else {
				return (await response.text()) as T;
			}
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
		console.error(error);
		if (error instanceof ApiError) {
			return { ok: false, status: error.status, resp: undefined, err: error.message };
		}
		return { ok: false, status: 500, resp: undefined, err: codes.errorUnknown };
	}
}

export function postLogin(username: string, password: string) {
	return api<Session>(`${baseURL}/forms/login`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ username, password })
	});
}

export function postRegister(username: string, password: string, confirmPassword: string) {
	return api<Session>(`${baseURL}/forms/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ username, password, confirmPassword })
	});
}

export function postUpdateUser(username: string, bio: string, country: string) {
	return api<Session>(`${baseURL}/forms/users`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ newUsername: username, newBio: bio, newCountry: country })
	});
}

export function postUpdatePassword(password: string, newPassword: string, confirmNewPassword: string) {
	return api<unknown>(`${baseURL}/forms/users/password`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ password, newPassword, confirmNewPassword })
	});
}

export function postCreateChallenge(timeControl: string, startColor: string, challengeeId: number) {
	return api<unknown>(`${baseURL}/forms/challenges/create`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ startColor, timeControl, challengeeId })
	});
}

export interface UpdateChallengeResp {
	gameId?: string;
}

export function postUpdateChallenge(challengerId: number, challengeeId: number, action: Action) {
	return api<UpdateChallengeResp>(`${baseURL}/forms/challenges/update`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ challengerId, challengeeId, action })
	});
}

export function postCreateGame(timeControl: string, firstColor: string) {
	return api<string>(`${baseURL}/forms/games/create`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ firstColor, timeControl })
	});
}

export function postLogout() {
	return api<unknown>(`${baseURL}/forms/logout`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include'
	});
}

export function postTempSession() {
	return api<string>(`${baseURL}/forms/session/temp`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include'
	});
}

interface RefreshResp {
	session: Session | null;
}

export function postRefresh() {
	return api<RefreshResp>(`${baseURL}/forms/session/refresh`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include'
	});
}

export function getReplays(userId: number, afterId?: number, fetch?: typeof window.fetch) {
	const params = new URLSearchParams({ userId: userId.toString() });
	if (afterId) {
		params.set('afterId', afterId.toString());
	}

	return api<Replay[]>(`${baseURL}/views/replays?${params.toString()}`, { method: 'GET' }, fetch);
}

export function getChallenges(participants: string, fetch?: typeof window.fetch) {
	const params = new URLSearchParams({ participants: participants });

	return api<Challenge[]>(
		`${baseURL}/views/challenges?${params.toString()}`,
		{
			method: 'GET',
			credentials: 'include'
		},
		fetch
	);
}

export interface LeaderboardResp {
	totalPages: number;
	userList: User[];
}

export function getLeaderboard(page: number, fetch?: typeof window.fetch) {
	const params = new URLSearchParams({ page: String(page) });

	return api<LeaderboardResp>(`${baseURL}/views/leaderboard?${params}`, { method: 'GET' }, fetch);
}

export function getProfile(fetch?: typeof window.fetch) {
	return api<User>(
		`${baseURL}/views/players/self`,
		{
			method: 'GET',
			credentials: 'include'
		},
		fetch
	);
}

export function getUserWithReplays(id: string, fetch?: typeof window.fetch) {
	return api<UserWithReplays>(`${baseURL}/views/players/${id}`, { method: 'GET' }, fetch);
}

export function getSearchPlayers(username: string, page?: number, fetch?: typeof window.fetch) {
	const params = new URLSearchParams({ username });
	if (page) {
		params.set('page', String(page));
	}

	return api<User[]>(`${baseURL}/views/players/search?${params.toString()}`, { method: 'GET' }, fetch);
}

export function getReplay(id: string, fetch?: typeof window.fetch) {
	return api<Replay>(`${baseURL}/views/replay/${id}`, { method: 'GET' }, fetch);
}

export function getReplayMoveList(id: string, fetch?: typeof window.fetch) {
	return api<PieceMove[]>(`${baseURL}/views/replay/${id}/move-list`, { method: 'GET' }, fetch);
}

export function cached<T>(get: (fetch?: typeof window.fetch) => Promise<T>) {
	let cache: T | undefined = undefined;
	return async (fetch?: typeof window.fetch) => {
		if (cache !== undefined) {
			return cache;
		}
		try {
			const countryList = await get(fetch);
			if (!countryList) {
				return undefined;
			}
			cache = countryList;
			return cache;
		} catch (error) {
			console.error(error);
			return undefined;
		}
	};
}

export const getCountries = cached(async (fetch?: typeof window.fetch) => {
	return api<string[]>(`${baseURL}/views/countries`, { method: 'GET' }, fetch)
});

export const getInitialBoard = cached(async (fetch?: typeof window.fetch) => {
	return api<ChessBoard>(`${baseURL}/views/initial-board`, { method: 'GET' }, fetch);
});

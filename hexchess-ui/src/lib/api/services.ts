import { codes } from '$lib/utils/error';
import type { Action, ChallengeModel, ChessModel, MoveListModel, ReplayModel, ServiceModel, SessionModel, UserModel, UserWithReplaysModel } from '$lib/api/model';

export const appBaseURL = 'http://localhost:5173';
export const baseURL = 'http://localhost:8081';

export type Result<T> = [T | undefined, ServiceModel | undefined];
export type FetchFn = typeof window.fetch;
export type RequestFn<T> = (fetch?: FetchFn) => Promise<Result<T>>;

export async function request<Response extends object | unknown>(input: RequestInfo | URL, init?: RequestInit, request?: FetchFn): Promise<Result<Response>> {
	if (!request) {
		request = fetch;
	}
	try {
		if (init) {
			init.credentials = 'include';
			init.headers = { 'Content-Type': 'application/json' };
		}
		const response = await request(input, init);

		if (!response.ok) {
			const error: ServiceModel = await response.json();
			return [undefined, error];
		} else {
			const data: Response = await response.json();
			return [data, undefined];
		}
	} catch (error) {
		console.error(error);
		return [undefined, { status: 500, message: codes.errorUnknown }];
	}
}

export function cached<Response>(get: RequestFn<Response>): RequestFn<Response> {
	let cache: Response | undefined = undefined;
	return async (fetch?: FetchFn) => {
		if (cache !== undefined) {
			return [cache, undefined];
		}
		try {
			const [data, err] = await get(fetch);
			if (data) {
				cache = data;
			}
			if (err) {
				return [undefined, err];
			}
			return [cache, undefined];
		} catch (error) {
			console.error(error);
			return [undefined, { status: 500, message: codes.errorUnknown }];
		}
	};
}

export default {
	postLogin: (username: string, password: string) => {
		return request<SessionModel>(`${baseURL}/forms/login`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({
				username,
				password,
			})
		});
	},

	postRegister: (username: string, password: string, confirmPassword: string) => {
		return request<SessionModel>(`${baseURL}/forms/register`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({
				username,
				password,
				confirmPassword,
			})
		});
	},

	postUpdateUser: (username: string, bio: string, country: string) => {
		return request<SessionModel>(`${baseURL}/forms/users`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({
				newUsername: username,
				newBio: bio,
				newCountry: country,
			})
		});
	},

	postUpdatePassword: (password: string, newPassword: string, confirmNewPassword: string) => {
		return request<unknown>(`${baseURL}/forms/users/password`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({
				password,
				newPassword,
				confirmNewPassword,
			})
		});
	},

	postCreateChallenge: (timeControl: string, startColor: string, challengeeId: number) => {
		return request<unknown>(`${baseURL}/forms/challenges/create`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({
				startColor,
				timeControl,
				challengeeId,
			})
		});
	},

	postUpdateChallenge: (challengerId: number, challengeeId: number, action: Action) => {
		interface Response {
			gameId?: string;
		}

		return request<Response>(`${baseURL}/forms/challenges/update`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({ challengerId, challengeeId, action: action.toUpperCase() })
		});
	},

	postCreateGame: (timeControl: string, firstColor: string) => {
		interface Response {
			gameId?: string;
		}

		return request<Response>(`${baseURL}/forms/games/create`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({ firstColor, timeControl })
		});
	},

	postLogout: () => {
		return request<unknown>(`${baseURL}/forms/logout`, {
			method: 'POST',
			credentials: 'include'
		});
	},

	postTempSession: () => {
		interface Response {
			sessionId?: string;
		}

		return request<Response>(`${baseURL}/forms/session/temp`, {
			method: 'POST',
			credentials: 'include'
		});
	},

	postRefresh: () => {
		interface Response {
			session: SessionModel | null;
		}

		return request<Response>(`${baseURL}/forms/session/refresh`, {
			method: 'POST',
			credentials: 'include'
		});
	},

	getReplays: (userId: number, afterId?: number, fetch?: FetchFn) => {
		const params = new URLSearchParams({ userId: userId.toString() });
		if (afterId) {
			params.set('afterId', afterId.toString());
		}

		return request<ReplayModel[]>(`${baseURL}/views/replays?${params}`, { method: 'GET' }, fetch);
	},

	getChallenges: (participants: string, fetch?: FetchFn) => {
		const params = new URLSearchParams({ participants });
		return request<ChallengeModel[]>(`${baseURL}/views/challenges?${params}`, { method: 'GET' }, fetch);
	},

	getLeaderboard: (page: number, fetch?: FetchFn) => {
		const params = new URLSearchParams({ page: String(page) });

		interface Response {
			totalPages: number;
			userList: UserModel[];
		}

		return request<Response>(`${baseURL}/views/leaderboard?${params}`, { method: 'GET' }, fetch);
	},

	getProfile: (fetch?: FetchFn) => {
		return request<UserModel>(`${baseURL}/views/players/self`, { method: 'GET' }, fetch);
	},

	getUserWithReplays: (id: string, fetch?: FetchFn) => {
		return request<UserWithReplaysModel>(`${baseURL}/views/players/${id}`, { method: 'GET' }, fetch);
	},

	getSearchPlayers: (username: string, page?: number, fetch?: FetchFn) => {
		const params = new URLSearchParams({ username });
		if (page) {
			params.set('page', String(page))
		}
		return request<UserModel[]>(`${baseURL}/views/players/search?${params}`, { method: 'GET' }, fetch);
	},

	getReplay: (id: string, fetch?: FetchFn) => {
		return request<ReplayModel>(`${baseURL}/views/replay/${id}`, { method: 'GET' }, fetch);
	},

	getReplayMoveList: (id: string, fetch?: FetchFn)  =>  {
		return request<MoveListModel>(`${baseURL}/views/replay/${id}/moves`, { method: 'GET' }, fetch);
	},

	getChessRooms: (count: number, page?: number, fetch?: FetchFn) => {
		const params = new URLSearchParams({ count: String(count) });
		if (page) {
			params.set('page', String(page))
		}

		interface Response {
			chessList: ChessModel[];
			selfChessList: ChessModel[];
		}

		return request<Response>(`${baseURL}/views/chess/rooms?${params}`, { method: 'GET' }, fetch);
	},

	getCountries: cached(async (fetch?: FetchFn) => {
		return request<string[]>(`${baseURL}/views/countries`, { method: 'GET' }, fetch);
	})
};
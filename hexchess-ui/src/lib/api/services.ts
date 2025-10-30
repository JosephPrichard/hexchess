import { codes } from '$lib/utils/error';
import type { Action, ChallengeModel, ChessModel, MoveListModel, ReplayModel, ServiceModel, SessionModel, UserModel, FullUserModel } from '$lib/api/model';
import { v4 as uuidv4 } from 'uuid';
import { env } from '$env/dynamic/public';

export function appBaseURL() {
	return env.PUBLIC_APP_BASE_URL || 'http://localhost:5173';
}

export function baseURL() {
	return env.PUBLIC_BASE_URL || 'http://localhost:8081/api';
}

export type Result<T> = [T | undefined, ServiceModel | undefined];
export type FetchFn = typeof window.fetch;
export type RequestFn<T> = (fetch?: FetchFn) => Promise<Result<T>>;

export async function request<Response extends object | unknown>(input: RequestInfo | URL, init?: RequestInit, request?: FetchFn): Promise<Result<Response>> {
	if (!request) {
		request = fetch;
	}
	try {
		const trace = uuidv4();
		if (init) {
			init.credentials = 'include';
			init.headers = { 'Content-Type': 'application/json', 'X-trace': trace };
		}
		console.log(`making request to ${input} with trace ${trace}`);
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
		return request<SessionModel>(`${baseURL()}/login`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({
				username,
				password,
			})
		});
	},

	postRegister: (username: string, password: string, confirmPassword: string) => {
		return request<SessionModel>(`${baseURL()}/register`, {
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
		return request<SessionModel>(`${baseURL()}/users`, {
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
		return request<unknown>(`${baseURL()}/users/password`, {
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
		return request<unknown>(`${baseURL()}/challenges/create`, {
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
		return request<Response>(`${baseURL()}/challenges/update`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({ challengerId, challengeeId, action: action.toUpperCase() })
		});
	},

	postCreateGame: (timeControl: string, firstColor: string) => {
		interface Response {
			gameId?: string;
		}
		return request<Response>(`${baseURL()}/games/create`, {
			method: 'POST',
			credentials: 'include',
			body: JSON.stringify({ firstColor, timeControl })
		});
	},

	postLogout: () => {
		return request<unknown>(`${baseURL()}/logout`, {
			method: 'POST',
			credentials: 'include'
		});
	},

	postTempSession: () => {
		interface Response {
			sessionId?: string;
		}

		return request<Response>(`${baseURL()}/session/temp`, {
			method: 'POST',
			credentials: 'include'
		});
	},

	postRefresh: () => {
		interface Response {
			session: SessionModel | null;
		}
		return request<Response>(`${baseURL()}/session/refresh`, {
			method: 'POST',
			credentials: 'include'
		});
	},

	getReplays: (userId: number, afterId?: number, fetch?: FetchFn) => {
		interface Response {
			replayList: ReplayModel[];
		}
		const params = new URLSearchParams({ userId: userId.toString() });
		if (afterId) {
			params.set('afterId', afterId.toString());
		}
		return request<Response>(`${baseURL()}/replays?${params}`, { method: 'GET' }, fetch);
	},

	getChallenges: (participants: string, fetch?: FetchFn) => {
		interface Response {
			challengeList: ChallengeModel[];
		}
		const params = new URLSearchParams({ participants });
		return request<Response>(`${baseURL()}/challenges?${params}`, { method: 'GET' }, fetch);
	},

	getLeaderboard: (page: number, fetch?: FetchFn) => {
		interface Response {
			totalPages: number;
			userList: UserModel[];
		}
		const params = new URLSearchParams({ page: String(page) });
		return request<Response>(`${baseURL()}/leaderboard?${params}`, { method: 'GET' }, fetch);
	},

	getProfile: (fetch?: FetchFn) => {
		return request<UserModel>(`${baseURL()}/players/self`, { method: 'GET' }, fetch);
	},

	getUserWithReplays: (id: string, fetch?: FetchFn) => {
		const params = new URLSearchParams({ id: id });
		return request<FullUserModel>(`${baseURL()}/players?${params}`, { method: 'GET' }, fetch);
	},

	getSearchPlayers: (username: string, page?: number, fetch?: FetchFn) => {
		const params = new URLSearchParams({ username });
		if (page) {
			params.set('page', String(page))
		}
		interface Response {
			userList: UserModel[];
		}
		return request<Response>(`${baseURL()}/players/search?${params}`, { method: 'GET' }, fetch);
	},

	getReplay: (id: string, fetch?: FetchFn) => {
		interface Response {
			replay:ReplayModel;
		}
		const params = new URLSearchParams({ id: id });
		return request<Response>(`${baseURL()}/replay?${params}`, { method: 'GET' }, fetch);
	},

	getReplayMoveList: (id: string, fetch?: FetchFn)  =>  {
		const params = new URLSearchParams({ id: id });
		return request<MoveListModel>(`${baseURL()}/replay/move-list?${params}`, { method: 'GET' }, fetch);
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

		return request<Response>(`${baseURL()}/chess/rooms?${params}`, { method: 'GET' }, fetch);
	},

	getCountries: cached(async (fetch?: FetchFn) => {
		return request<string[]>(`${baseURL()}/countries`, { method: 'GET' }, fetch);
	})
};
import { codes } from '$lib/utils/error';
import type { Action, ChallengeModel, Chat, ChessModel, EloBuckets, FullUserModel, LbdUserModel, ReplayModel, ServiceModel, SessionModel, UserModel } from './models';
import { v4 as uuidv4 } from 'uuid';
import { env } from '$env/dynamic/public';
import { MoveHistory } from '$lib/pb/messages';

export function appBaseURL() {
	return env.PUBLIC_APP_BASE_URL || 'http://localhost:5173';
}

export function baseURL() {
	return env.PUBLIC_BASE_URL || 'http://localhost:8081/api';
}

export type Result<T> = [T | undefined, ServiceModel | undefined];
export type FetchFn = typeof window.fetch;
export type RequestFn<T> = (fetch?: FetchFn) => Promise<Result<T>>;

export async function requestJSON<Response extends object | {}>(input: RequestInfo | URL, init?: RequestInit, request?: FetchFn): Promise<Result<Response>> {
	if (!request) {
		request = fetch;
	}
	try {
		const trace = uuidv4();
		if (!init) {
			init = {};
		}
		if (init) {
			init.credentials = 'include';
			init.headers = { 'Content-Type': 'application/json', 'X-trace': trace };
		}
		console.log(`sending request to ${input} with trace ${trace}`);
		const response = await request(input, init);

		if (!response.ok) {
			const error: ServiceModel = await response.json();
			return [undefined, error];
		} else {
			const data: Response = await response.json();
			return [data, undefined];
		}
	} catch (error) {
		if (!(error instanceof TypeError)) {
			console.error(error);
		}
		return [undefined, { status: 500, message: "", errors: codes.errorUnknown }];
	}
}

export async function requestBlob(input: RequestInfo | URL, init?: RequestInit, request?: FetchFn): Promise<Result<ArrayBuffer>> {
	if (!request) {
		request = fetch;
	}
	try {
		const trace = uuidv4();
		if (!init) {
			init = {};
		}
		if (init) {
			init.credentials = 'include';
			init.headers = { 'X-trace': trace };
		}
		console.log(`sending request to ${input} with trace ${trace}`);
		const response = await request(input, init);

		if (!response.ok) {
			const error: ServiceModel = await response.json();
			return [undefined, error];
		} else {
			const data = await response.arrayBuffer();
			return [data, undefined];
		}
	} catch (error) {
		if (!(error instanceof TypeError)) {
			console.error(error);
		}
		return [undefined, { status: 500, errors: codes.errorUnknown }];
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
			return [undefined, { status: 500, errors: codes.errorUnknown }];
		}
	};
}

function postLogin(username: string, password: string) {
	return requestJSON<SessionModel>(`${baseURL()}/login`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ username, password }),
	});
}


function postGoogleLogin(token: string) {
	return requestJSON<SessionModel>(`${baseURL()}/login/google`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ token }),
	});
}

function postRegister(username: string, password: string, confirmPassword: string) {
	return requestJSON<SessionModel>(`${baseURL()}/register`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ username, password, confirmPassword }),
	});
}

function postUpdateUser(username: string, bio: string, country: string) {
	return requestJSON<SessionModel>(`${baseURL()}/users`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({
			newUsername: username,
			newBio: bio,
			newCountry: country,
		}),
	});
}

function postUpdatePassword(password: string, newPassword: string, confirmNewPassword: string) {
	return requestJSON<{}>(`${baseURL()}/users/password`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ password, newPassword, confirmNewPassword }),
	});
}

function postCreateChallenge(mode: string, startColor: string, challengeeId: number) {
	return requestJSON<{}>(`${baseURL()}/challenges/create`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ startColor, mode, challengeeId }),
	});
}

function postUpdateChallenge(challengerId: number, challengeeId: number, action: Action) {
	interface Response {
		gameId?: string;
	}
	return requestJSON<Response>(`${baseURL()}/challenges/update`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ challengerId, challengeeId, action: action.toUpperCase() }),
	});
}

function postCreateGame(mode: string, firstColor: string, fen: string) {
	interface Response {
		gameId?: string;
	}
	return requestJSON<Response>(`${baseURL()}/games/create`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ firstColor, mode, fen }),
	});
}

function postLogout() {
	return requestJSON<{}>(`${baseURL()}/logout`, {
		method: 'POST',
		credentials: 'include',
	});
}

function postTempSession() {
	interface Response {
		sessionId?: string;
	}
	return requestJSON<Response>(`${baseURL()}/session/temp`, {
		method: 'POST',
		credentials: 'include',
	});
}

function postRefreshSession() {
	interface Response {
		session: SessionModel | null;
	}
	return requestJSON<Response>(`${baseURL()}/session/refresh`, {
		method: 'POST',
		credentials: 'include',
	});
}

export async function postProfilePic(file: File) {
	try {
		const input = `${baseURL()}/users/profile-pics`;

		const form = new FormData();
		form.append("file", file, file.name);

		const trace = uuidv4();
		console.log(`sending request to ${input} with trace ${trace}`);
		const response = await fetch(input, {
			method: "POST",
			body: form, // browser sets multipart boundary automatically
			credentials: 'include',
		});

		const data: ServiceModel = await response.json();
		if (!response.ok) {
			return [undefined, data];
		} else {
			return [data, undefined];
		}
	} catch (error) {
		if (!(error instanceof TypeError)) {
			console.error(error);
		}
		return [undefined, { status: 500,errors: codes.errorUnknown }];
	}
}

function getReplays(userId: number, afterId?: number, fetch?: FetchFn) {
	interface Response {
		replayList: ReplayModel[];
	}
	const params = new URLSearchParams({ userId: userId.toString() });
	if (afterId)
		params.set('afterId', afterId.toString());
	return requestJSON<Response>(`${baseURL()}/replays?${params}`, { method: 'GET' }, fetch);
}

function getChallenges(participants: string, fetch?: FetchFn) {
	interface Response {
		challengeList: ChallengeModel[];
	}
	const params = new URLSearchParams({ participants });
	return requestJSON<Response>(`${baseURL()}/challenges?${params}`, { method: 'GET' }, fetch);
}

function getLeaderboard(page: number, mode: string, fetch?: FetchFn) {
	interface Response {
		totalPages: number;
		userList: LbdUserModel[];
	}
	const params = new URLSearchParams({
		page: String(page),
		mode: mode,
	});
	return requestJSON<Response>(`${baseURL()}/leaderboard?${params}`, { method: 'GET' }, fetch);
}

function getProfile(fetch?: FetchFn) {
	return requestJSON<UserModel>(`${baseURL()}/players/self`, { method: 'GET' }, fetch);
}

function getUser(id: string, withReplays: boolean, fetch?: FetchFn) {
	const params = new URLSearchParams({ id, withReplays: withReplays.toString() });
	return requestJSON<FullUserModel>(`${baseURL()}/players?${params}`, { method: 'GET' }, fetch);
}

function getGameExistence(id: string, fetch?: FetchFn) {
	interface Response {
		message: string;
	}
	const params = new URLSearchParams({ gameId: id });
	return requestJSON<Response>(`${baseURL()}/game/rooms/exists?${params}`, { method: 'GET' }, fetch);
}

function getSearchPlayers(username: string, page?: number, fetch?: FetchFn) {
	const params = new URLSearchParams({ username });
	if (page)
		params.set('page', String(page));
	interface Response {
		userList: LbdUserModel[];
	}
	return requestJSON<Response>(`${baseURL()}/players/search?${params}`, { method: 'GET' }, fetch);
}

function getReplay(id: string, fetch?: FetchFn) {
	interface Response {
		replay: ReplayModel;
	}
	const params = new URLSearchParams({ id });
	return requestJSON<Response>(`${baseURL()}/replay?${params}`, { method: 'GET' }, fetch);
}

function getEloHistories(userId: number, timeframe: string, fetch?: FetchFn) {
	const params = new URLSearchParams({
		userId: userId.toString(),
		timeframe
	});
	interface Response {
		buckets: Record<string, EloBuckets>
	}
	return requestJSON<Response>(`${baseURL()}/replay/elo-histories?${params}`, { method: 'GET' }, fetch);
}

async function getReplayMoveHistory(id: string, fetch?: FetchFn): Promise<Result<MoveHistory>> {
	const params = new URLSearchParams({ replayId: id });
	const [buf, error] = await requestBlob(`${baseURL()}/replay/move-list?${params}`, { method: 'GET' }, fetch);
	if (buf) {
		const timeNow = performance.now();
		const result = MoveHistory.fromBinary(new Uint8Array(buf));

		const timeTaken = performance.now() - timeNow;
		console.log(`moveList deserialization took ${timeTaken}ms`);

		return [result, error];
	} else {
		return [undefined, error];
	}
}

function getGameRooms(count: number, page?: number, fetch?: FetchFn) {
	const params = new URLSearchParams({ count: String(count) });
	if (page) params.set('page', String(page));
	interface Response {
		chats: ChessModel[];
		selfChessList: ChessModel[];
	}
	return requestJSON<Response>(`${baseURL()}/game/rooms?${params}`, { method: 'GET' }, fetch);
}

function getGameChats(gameId: string) {
	const params = new URLSearchParams({ gameId: String(gameId) });
	interface Response {
		chats: Chat[];
	}
	return requestJSON<Response>(`${baseURL()}/game/rooms/chats?${params}`, { method: 'GET' }, fetch);
}

const getCountries = cached(async (fetch?: FetchFn) => {
	return requestJSON<string[]>(`${baseURL()}/countries`, { method: 'GET' }, fetch);
});

export default {
	postLogin,
	postGoogleLogin,
	postRegister,
	postUpdateUser,
	postUpdatePassword,
	postCreateChallenge,
	postUpdateChallenge,
	postCreateGame,
	postLogout,
	postTempSession,
	postRefreshSession,
	postProfilePic,
	getReplays,
	getChallenges,
	getLeaderboard,
	getProfile,
	getUser,
	getGameExistence,
	getSearchPlayers,
	getReplay,
	getReplayMoveHistory,
	getChessRooms: getGameRooms,
	getCountries,
	getEloHistories,
};
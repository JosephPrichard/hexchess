import { logger } from '$lib/utils/logger';
import { codes} from '$lib/utils/error';
import type { Action, Challenge, ChessMetadata, EloBuckets, FullPlayer, LeaderboardUser, Replay, ServiceResponse, Session, User } from './models';
import { v4 as uuidv4 } from 'uuid';
import { env as publicEnv } from '$env/dynamic/public';
import { ChatMessages, MoveHistory } from '$lib/pb/messages';
import { createSHA256 } from "hash-wasm";
import { browser } from '$app/environment';
import globals from './globals';

const defaultFrontend = 'http://localhost:5173';
const defaultBackend = 'http://localhost:8080/api';
const maxTimeoutMs = 5_000; // strict 5 second timeout

export function frontendBaseURL() {
	// from browser to server serving frontend js/html/css files (browser to public hostname)
	return publicEnv.PUBLIC_APP_BASE_URL || defaultFrontend;
}

export function backendBaseURL() {
	if (browser) {
		// from browser to server serving backend JSON API (browser to public hostname)
		return publicEnv.PUBLIC_APP_BASE_URL + "/api" || defaultBackend;
	} else {
		// from NODE.js server to server serving backend JSON API (within same datacenter)
		return globals.internalBackendBaseURL + "/api" || defaultBackend;
	}
}

export type Result<T> = [T | undefined, ServiceResponse | undefined];
export type FetchFn = typeof window.fetch;
export type RequestFn<T> = (fetch?: FetchFn) => Promise<Result<T>>;

const unknownError = () => ({ status: 500, message: "", error: codes.errorUnknown, errors: {} });

export async function requestJSON<Response extends object | {}>(input: RequestInfo | URL, init?: RequestInit, request?: FetchFn): Promise<Result<Response>> {
	try {
		if (!request) {
			request = fetch;
		}

		const startTime = Date.now();

		const trace = uuidv4();
		if (!init) {
			init = {};
		}
		init.signal = AbortSignal.timeout(maxTimeoutMs);
		init.credentials = 'include';
		init.headers = { 'Content-Type': 'application/json', 'X-trace': trace };

		logger.info("sending request", { kind: "JSON", trace, input, init });
		const response = await request(input, init);

		if (!response.ok) {
			const errorMessage: string = await response.text();
			logger.warn("received error response", { kind: "JSON", trace, response: errorMessage || "" });
			return [undefined, JSON.parse(errorMessage) as ServiceResponse];
		} else {
			const data: Response = await response.json();
			logger.info("handled request", { kind: "JSON", trace, timeTaken: Date.now() - startTime });
			return [data, undefined];
		}
	} catch (error) {
		if (!(error instanceof TypeError)) {
			logger.error("failed to send http request", error);
		}
		return [undefined, unknownError()];
	}
}

export async function requestBlob(input: RequestInfo | URL, init?: RequestInit, request?: FetchFn): Promise<Result<ArrayBuffer>> {
	try {
		if (!request) {
			request = fetch;
		}

		const startTime = Date.now();

		const trace = uuidv4();
		if (!init) {
			init = {};
		}
		init.signal = AbortSignal.timeout(maxTimeoutMs);
		init.credentials = 'include';
		init.headers = { 'X-trace': trace };

		logger.info("sending request", { kind: "BLOB", input, trace });
		const response = await request(input, init);

		if (!response.ok) {
			const errorMessage: string = await response.text();
			logger.warn("received error response", { kind: "BLOB", trace, response: errorMessage || "" });
			return [undefined, JSON.parse(errorMessage) as ServiceResponse];
		} else {
			const data = await response.arrayBuffer();
			logger.info("handled request", { kind: "BLOB", trace, timeTaken: Date.now() - startTime });
			return [data, undefined];
		}
	} catch (error) {
		if (!(error instanceof TypeError)) {
			logger.error("failed to send http request", error);
		}
		return [undefined, unknownError()];
	}
}

export function memcached<Response>(get: RequestFn<Response>): RequestFn<Response> {
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
			logger.error("failed to send http request", error);
			return [undefined, unknownError()];
		}
	};
}

function postLogin(username: string, password: string) {
	return requestJSON<Session>(`${backendBaseURL()}/login`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ username, password }),
	});
}


function postGoogleLogin(token: string) {
	return requestJSON<Session>(`${backendBaseURL()}/login/google`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ token }),
	});
}

function postRegister(username: string, password: string, confirmPassword: string) {
	return requestJSON<Session>(`${backendBaseURL()}/register`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ username, password, confirmPassword }),
	});
}

function postUpdateUser(username: string, bio: string, country: string) {
	return requestJSON<Session>(`${backendBaseURL()}/users`, {
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
	return requestJSON<{}>(`${backendBaseURL()}/users/password`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ password, newPassword, confirmNewPassword }),
	});
}

function postCreateChallenge(mode: string, startColor: string, challengeeId: number) {
	return requestJSON<{}>(`${backendBaseURL()}/challenges/create`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ startColor, mode, challengeeId }),
	});
}

function postUpdateChallenge(challengerId: number, challengeeId: number, action: Action) {
	interface Response {
		gameId?: string;
	}
	return requestJSON<Response>(`${backendBaseURL()}/challenges/update`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ challengerId, challengeeId, action: action.toUpperCase() }),
	});
}

function postCreateGame(mode: string, firstColor: string, fen: string) {
	interface Response {
		gameId?: string;
	}
	return requestJSON<Response>(`${backendBaseURL()}/games/create`, {
		method: 'POST',
		credentials: 'include',
		body: JSON.stringify({ firstColor, mode, fen }),
	});
}

function postLogout() {
	return requestJSON<{}>(`${backendBaseURL()}/logout`, {
		method: 'POST',
		credentials: 'include',
	});
}

function postTempSession() {
	interface Response {
		sessionId?: string;
	}
	return requestJSON<Response>(`${backendBaseURL()}/session/temp`, {
		method: 'POST',
		credentials: 'include',
	});
}

function postRefreshSession() {
	interface Response {
		session: Session | null;
	}
	return requestJSON<Response>(`${backendBaseURL()}/session/refresh`, {
		method: 'POST',
		credentials: 'include',
	});
}


export async function postProfilePic(file: File): Promise<Result<{}>> {
	try {
		const profilePicUrl = `${backendBaseURL()}/users/profile-pics`;

		const trace = uuidv4();
		logger.info(`sending request to ${profilePicUrl} with trace ${trace}`);

		const hasher = await createSHA256();
		const reader = file.stream().getReader();
		while (true) {
			const { done, value } = await reader.read();
			if (done) break;
			hasher.update(value);
		}
		const contentHash = btoa(String.fromCharCode(...hasher.digest('binary')));

		logger.info("computed content hash for upload:", contentHash);

		const profileResp = await fetch(profilePicUrl, {
			method: "POST",
			credentials: 'include',
			body: file,
			headers: {
				"Content-Digest": contentHash,
				"Content-Length": String(file.size),
				"Content-Type": file.type,
				"X-trace": trace,
			},
		});

		if (!profileResp.ok) {
			const data: ServiceResponse = await profileResp.json();
			return [undefined, data];
		} else {
			return [{}, undefined];
		}
	} catch (error) {
		if (!(error instanceof TypeError)) {
			logger.error("fatal http error", error);
		}
		return [undefined, unknownError()];
	}
}

export interface ReplaysQuery {
	userId?: string;
	whiteId?: string;
	blackId?: string;
	winnerId?: string;
	loserId?: string;

	whitename?: string;
	blackname?: string;
	winnername?: string;
	losername?: string;

	fromDate?: string;
	toDate?: string;
	mode?: string;
	result?: string;
	cause?: string;
	afterId?: string;
	afterTurnCount?: string;
	afterRating?: string;

	sort?: string;
}

function getReplays(replaysQuery: ReplaysQuery, fetch?: FetchFn) {
	interface Response {
		replayList: Replay[];
	}

	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(replaysQuery)) {
		if (value !== undefined) {
			params.set(key, String(value));
		}
	}

	return requestJSON<Response>(`${backendBaseURL()}/replays?${params}`, { method: 'GET' }, fetch);
}

function getChallenges(participants: string, fetch?: FetchFn) {
	interface Response {
		challengeList: Challenge[];
	}
	const params = new URLSearchParams({ participants });
	return requestJSON<Response>(`${backendBaseURL()}/challenges?${params}`, { method: 'GET' }, fetch);
}

function getChallengesCount(fetch?: FetchFn) {
	interface Response {
		count: number;
	}
	return requestJSON<Response>(`${backendBaseURL()}/challenges/count`, { method: 'GET' }, fetch);
}

function getLeaderboard(page: number, mode: string, fetch?: FetchFn) {
	interface Response {
		totalPages: number;
		userList: LeaderboardUser[];
	}
	const params = new URLSearchParams({
		page: String(page),
		mode: mode,
	});
	return requestJSON<Response>(`${backendBaseURL()}/leaderboard?${params}`, { method: 'GET' }, fetch);
}

function getProfile(fetch?: FetchFn) {
	return requestJSON<User>(`${backendBaseURL()}/players/self`, { method: 'GET' }, fetch);
}

function getUser(id: string, withReplays: boolean, fetch?: FetchFn) {
	const params = new URLSearchParams({ id, withReplays: String(withReplays) });
	return requestJSON<FullPlayer>(`${backendBaseURL()}/players?${params}`, { method: 'GET' }, fetch);
}

function getGameExistence(id: string, fetch?: FetchFn) {
	interface Response {
		message: string;
	}
	const params = new URLSearchParams({ gameId: id });
	return requestJSON<Response>(`${backendBaseURL()}/game/rooms/exists?${params}`, { method: 'GET' }, fetch);
}

function getSearchPlayers(username: string, page?: number, fetch?: FetchFn, signal?: AbortSignal) {
	const params = new URLSearchParams({ username });
	if (page)
		params.set('page', String(page));
	interface Response {
		userList?: LeaderboardUser[];
	}
	return requestJSON<Response>(`${backendBaseURL()}/players/search?${params}`, { method: 'GET', signal }, fetch);
}

function getReplay(id: string, idKind = "BY_REPLAY_ID", fetch?: FetchFn) {
	interface Response {
		replay: Replay;
	}
	const params = new URLSearchParams({ id, idKind });
	return requestJSON<Response>(`${backendBaseURL()}/replay?${params}`, { method: 'GET' }, fetch);
}

function getEloHistories(userId: number, timeframe: string, fetch?: FetchFn) {
	interface Response {
		buckets: Record<string, EloBuckets>
	}
	const params = new URLSearchParams({
		userId: userId.toString(),
		timeframe
	});
	return requestJSON<Response>(`${backendBaseURL()}/replay/elo-histories?${params}`, { method: 'GET' }, fetch);
}

async function getReplayMoveHistory(id: string, fetch?: FetchFn): Promise<Result<MoveHistory>> {
	const params = new URLSearchParams({ replayId: id });
	const [buf, err] = await requestBlob(`${backendBaseURL()}/replay/move-list?${params}`, { method: 'GET' }, fetch);
	if (buf) {
		const result = timed("moveHistory", () => MoveHistory.fromBinary(new Uint8Array(buf)));
		return [result, undefined];
	} else {
		return [undefined, err];
	}
}

function getGameRooms(count: number, page?: number, fetch?: FetchFn) {
	interface Response {
		chessList: ChessMetadata[];
		selfChessList: ChessMetadata[];
	}
	const params = new URLSearchParams({ count: String(count) });
	if (page) params.set('page', String(page));
	return requestJSON<Response>(`${backendBaseURL()}/game/rooms?${params}`, { method: 'GET' }, fetch);
}

async function getGameChats(gameId: string): Promise<Result<ChatMessages>> {
	const params = new URLSearchParams({ gameId: String(gameId) });
	const [buf, err] = await requestBlob(`${backendBaseURL()}/game/rooms/chats?${params}`, { method: 'GET' }, fetch);
	if (buf) {
		const result = timed("gameChats", () => ChatMessages.fromBinary(new Uint8Array(buf)));
		return [result, undefined];
	} else {
		return [undefined, err];
	}
}

async function getIsUserActive(userId: string | number) {
	interface Response {
		isUserActive: boolean;
	}
	const params = new URLSearchParams({ userId: String(userId) });
	return requestJSON<Response>(`${backendBaseURL()}/players/activity?${params}`, { method: 'GET' }, fetch);
}

const getCountries = memcached(async (fetch?: FetchFn) => {
	return requestJSON<string[]>(`${backendBaseURL()}/countries`, { method: 'GET' }, fetch);
});

function timed<Result>(name: string, work: () => Result): Result {
	const timeNow = performance.now();
	const result = work();

	const timeTaken = performance.now() - timeNow;
	logger.info(`${name} deserialization took ${timeTaken}ms`);

	return result;
}

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
	getChallengesCount,
	getLeaderboard,
	getProfile,
	getUser,
	getSearchPlayers,
	getReplay,
	getReplayMoveHistory,
	getGameExistence,
	getGameRooms,
	getGameChats,
	getCountries,
	getEloHistories,
	getIsUserActive
};
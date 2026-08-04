import { logger } from '$lib/utils/logger';
import { codes} from '$lib/utils/error';
import type { Action, Challenge, ChessMetadata, EloBuckets, FullPlayer, LeaderboardUser, Replay, ServiceResponse, Session, User } from './models';
import { v4 as uuidv4 } from 'uuid';
import { env as publicEnv } from '$env/dynamic/public';
import { ChatMessages, MoveHistory } from '$lib/pb/messages';
import { createSHA256 } from "hash-wasm";
import { browser } from '$app/environment';
import globals from './globals';
import type { ReplaysQuery } from "$lib/api/bodies";

const defaultFrontend = 'http://localhost:5173';
const defaultBackend = 'http://localhost:8080';

// from browser to server serving backend JSON API (browser to public hostname)
const clientBackendURL = (publicEnv.PUBLIC_APP_BASE_URL || defaultBackend) + "/api";

// from NODE.js server to server serving backend JSON API (within same datacenter)
const serverBackendURL = (globals.internalBackendBaseURL || defaultBackend) + "/api";

const maxTimeoutMs = 5_000; // strict 5 second timeout

export function frontendBaseURL() {
	// from browser to server serving frontend js/html/css files (browser to public hostname)
	return publicEnv.PUBLIC_APP_BASE_URL || defaultFrontend;
}

export function backendBaseURL() {
	return browser ? clientBackendURL : serverBackendURL;
}

export interface PageLoadEvent {
  	request?: Request;
	data?: { cookieHeader: string }
}

export function rollout() {
	return publicEnv.PUBLIC_ROLLOUT || ""
}

export type Result<T> = [T | undefined, ServiceResponse | undefined];
export type FetchFn = typeof window.fetch;
export type RequestFn<T> = (fetch?: FetchFn) => Promise<Result<T>>;

interface ResponseMap<JSONResponse extends object | {}> {
	["JSON"]: JSONResponse;
	["BLOB"]: ArrayBuffer;
}

const responseMap = {
	"JSON": (response: Response) => response.json(),
	"BLOB": (response: Response) => response.arrayBuffer()
};

export async function doRequest<Kind extends "JSON" | "BLOB", JSONResponse extends object | {} = {}>(
	kind: Kind,
	input: RequestInfo | URL,
	init?: RequestInit,
	customFetch?: FetchFn
): Promise<Result<ResponseMap<JSONResponse>[Kind]>> {
	try {
		const startTime = Date.now();
		const trace = uuidv4();

		if (!customFetch) {
			customFetch = fetch;
		}
		if (!init) {
			init = {};
		}
		if (!init.headers) {
			init.headers = {};
		}

		init.signal = AbortSignal.timeout(maxTimeoutMs);
		init.credentials = 'include';
		init.headers = { ...init.headers, 'X-trace': trace, rollout: rollout() };

		logger.info("sending request", { kind, input, trace });

		const response = await customFetch(input, init);
		
		if (!response.ok) {
			const errorMessage: string = await response.text();
			logger.warn("received error response", { kind, trace, response: errorMessage || "" });

			return [undefined, JSON.parse(errorMessage) as ServiceResponse];
		} else {
			const data = await responseMap[kind](response);
			logger.info("handled request", { kind, trace, timeTaken: Date.now() - startTime });

			return [data as ResponseMap<JSONResponse>[Kind], undefined];
		}
	} catch (error) {
		logger.error("failed to send http request", { kind, input }, error);
		return [undefined, ({ status: 500, message: "", error: codes.errorUnknown, errors: {} })];
	}
}

export async function requestJSON<Response extends object | {}>(input: RequestInfo | URL, init?: RequestInit, request?: FetchFn): Promise<Result<Response>> {
	return doRequest<"JSON", Response>("JSON", input, init, request);
}

export async function requestBlob(input: RequestInfo | URL, init?: RequestInit, request?: FetchFn): Promise<Result<ArrayBuffer>> {
	return doRequest("BLOB", input, init, request);
}

export function cache<Response>(get: RequestFn<Response>): RequestFn<Response> {
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
			return [undefined, ({ status: 500, message: "", error: codes.errorUnknown, errors: {} })];
		}
	};
}

function timed<Result>(name: string, work: () => Result): Result {
	const timeNow = performance.now();
	const result = work();

	const timeTaken = performance.now() - timeNow;
	logger.info(`${name} took ${timeTaken}ms`);

	return result;
}

async function hashContent(file: File) {
	const hasher = await createSHA256();
	const reader = file.stream().getReader();
	while (true) {
		const { done, value } = await reader.read();
		if (done) break;
		hasher.update(value);
	}
	return btoa(String.fromCharCode(...hasher.digest('binary')));
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
	return requestJSON<Response>(`${backendBaseURL()}/users/profile-pics`, {
		method: "POST",
		credentials: 'include',
		body: file,
		headers: {
			"Content-Digest": await hashContent(file),
			"Content-Length": String(file.size),
			"Content-Type": file.type,
		},
	});
}

function getReplays(replaysQuery: ReplaysQuery) {
	interface Response {
		replayList: Replay[];
	}

	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(replaysQuery)) {
		if (value !== undefined) {
			params.set(key, String(value));
		}
	}

	return requestJSON<Response>(`${backendBaseURL()}/replays?${params}`, { method: 'GET' });
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

function getLeaderboard(page: number, mode: string) {
	interface Response {
		totalPages: number;
		userList: LeaderboardUser[];
	}
	const params = new URLSearchParams({
		page: String(page),
		mode: mode,
	});
	return requestJSON<Response>(`${backendBaseURL()}/leaderboard?${params}`, { method: 'GET' });
}

function getProfile(fetch?: FetchFn) {
	return requestJSON<User>(`${backendBaseURL()}/players/self`, { method: 'GET' }, fetch);
}

function getUser(id: string, withReplays: boolean) {
	const params = new URLSearchParams({ 
		id, 
		withReplays: String(withReplays)
	});
	return requestJSON<FullPlayer>(`${backendBaseURL()}/players?${params}`, { method: 'GET' });
}

function getGameExistence(id: string) {
	interface Response {
		message: string;
	}
	const params = new URLSearchParams({ gameId: id });
	return requestJSON<Response>(`${backendBaseURL()}/game/rooms/exists?${params}`, { method: 'GET' });
}

function getSearchPlayers(username: string, page?: number, signal?: AbortSignal) {
	const params = new URLSearchParams({ username });
	if (page)
		params.set('page', String(page));
	interface Response {
		userList?: LeaderboardUser[];
	}
	return requestJSON<Response>(`${backendBaseURL()}/players/search?${params}`, { method: 'GET', signal });
}

function getReplay(id: string, idKind = "BY_REPLAY_ID") {
	interface Response {
		replay: Replay;
	}
	const params = new URLSearchParams({ id, idKind });
	return requestJSON<Response>(`${backendBaseURL()}/replay?${params}`, { method: 'GET' });
}

function getEloHistories(userId: number, timeframe: string) {
	interface Response {
		buckets: Record<string, EloBuckets>
	}
	const params = new URLSearchParams({
		userId: userId.toString(),
		timeframe
	});
	return requestJSON<Response>(`${backendBaseURL()}/replay/elo-histories?${params}`, { method: 'GET' });
}

function getGameRooms(count: number, page?: number) {
	interface Response {
		chessList: ChessMetadata[];
		selfChessList: ChessMetadata[];
	}
	const params = new URLSearchParams({ count: String(count) });
	if (page) params.set('page', String(page));
	return requestJSON<Response>(`${backendBaseURL()}/game/rooms?${params}`, { method: 'GET' });
}

async function getReplayMoveHistory(id: string): Promise<Result<MoveHistory>> {
	const params = new URLSearchParams({ replayId: id });
	const [buf, err] = await requestBlob(`${backendBaseURL()}/replay/move-list?${params}`, { method: 'GET' });
	if (buf) {
		const result = timed("moveHistory deserialization", () => MoveHistory.fromBinary(new Uint8Array(buf)));
		return [result, undefined];
	} else {
		return [undefined, err];
	}
}

async function getGameChats(gameId: string): Promise<Result<ChatMessages>> {
	const params = new URLSearchParams({ gameId: String(gameId) });
	const [buf, err] = await requestBlob(`${backendBaseURL()}/game/rooms/chats?${params}`, { method: 'GET' });
	if (buf) {
		const result = timed("gameChats deserialization", () => ChatMessages.fromBinary(new Uint8Array(buf)));
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
	return requestJSON<Response>(`${backendBaseURL()}/players/activity?${params}`, { method: 'GET' });
}

const getCountries = cache(
	() => requestJSON<string[]>(`${backendBaseURL()}/countries`, { method: 'GET' })
);

export const services = {
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
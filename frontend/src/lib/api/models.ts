import type { ChatMessage, PlayerState, Replay } from '$lib/pb/messages';

function nameMapIntoOptions<T extends string>(nameMap: Record<T, string>) {
	return Object.entries(nameMap)
		.map(([key, value]) => ({label: value as string, value: key as T}))
}

export type Action = 'delete' | 'reject' | 'accept';

export type ColorSelect = 'RANDOM' | 'WHITE' | 'BLACK';

export type GameMode =
	| "TIMED_1+0"
	| "TIMED_3+2"
	| "TIMED_15+10"
	| "CORRESPONDENCE_1"
	| "CORRESPONDENCE_7"
	| "CORRESPONDENCE_14";

export const GameModeNameMap: Record<GameMode, string> = {
	"TIMED_1+0": "Bullet",
	"TIMED_3+2": "Blitz",
	"TIMED_15+10": "Rapid",
	"CORRESPONDENCE_1": "Correspondence 1d",
	"CORRESPONDENCE_7": "Correspondence 7d",
	"CORRESPONDENCE_14": "Correspondence 14d",
};

export const UntypedGameModeNameMap = GameModeNameMap as Record<string, string>;

export const GameModeOptions = nameMapIntoOptions(GameModeNameMap);

export type ReplayCause =
	| "CHECKMATE"
	| "STALEMATE"
	| "FORFEIT";

export const ReplayCauseNameMap: Record<ReplayCause, string> = {
	"CHECKMATE": "Checkmate",
	"STALEMATE": "Stalemate",
	"FORFEIT": "Forfeit",
};

export const ReplayCauseOptions = nameMapIntoOptions(ReplayCauseNameMap);

export type ReplayResult =
	| "WHITE_WINS"
	| "BLACK_WINS"
	| "DRAW";

export const ReplayResultNameMap: Record<ReplayResult, string> = {
	"WHITE_WINS": "White Victory",
	"BLACK_WINS": "Black Victory",
	"DRAW": "Draw",
};

export const ReplayResultOptions = nameMapIntoOptions(ReplayResultNameMap);

export type ReplayQuerySortKey =
	| "id"
	| "turnCount"
	| "rating";

export const ReplayQuerySortKeyNameMap: Record<ReplayQuerySortKey, string> = {
	"id": "Played On",
	"turnCount": "Turn Count",
	"rating": "Rating",
};

export const ReplayQuerySortKeyOptions = nameMapIntoOptions(ReplayQuerySortKeyNameMap);

const msPerMin = 60_000;

export const GameModeTimers: Record<string, number> = {
	"TIMED_1+0": msPerMin,
	"TIMED_3+2": 3 * msPerMin,
	"TIMED_15+10": 15 * msPerMin,
};

export interface SessionModel {
	id: number;
	username: string;
	country: string;
	elo: number;
	ttlSecs: number | null;
}

export type UserModel = {
	id: number;
	username: string;
	country: string;
	rank: number;
	bio: string;
	joinedOn: string;
}

export function isGuestUser(user: PlayerState | PlayerModel) {
	return user.id < 0;
}

export type LbdUserModel = UserModel & {
	elo: number;
	highestElo: number;
	wins: number;
	losses: number;
	draws: number;
	winrate: number;
}

export interface UserStatsEntity {
	totalWins: number;
	totalLosses: number;
	totalDraws: number;
	avgElo: number;
	highestElo: number;
	totalWinrate: number;
	modeStats: {
		mode: GameMode;
		rank: number;
		wins: number;
		draws: number;
		losses: number;
		winrate: number;
		elo: number;
		highestElo: number;
	}[];
}

export interface ChallengeModel {
	challengerId: number;
	challengerName: string;
	challengerCountry: string;
	challengerElo: number;
	challengeeId: number;
	challengeeName: string;
	challengeeCountry: string;
	challengeeElo: number;
	mode: string;
	madeOn: string;
	expiresOn: string;
}

export interface ReplayModel {
	id: number;
	whiteId: number;
	blackId: number;
	whiteName: string;
	blackName: string;
	whiteCountry: string;
	blackCountry: string;
	whiteElo: number;
	blackElo: number;
	playedOn: string;
	result: string;
	cause: string;
	mode: string;
	whiteEloDiff: number;
	blackEloDiff: number;
	winEloDiff: number;
	loseEloDiff: number;
	rating?: number;
	turnCount?: number;
}

export function mapReplay(replay: Replay): ReplayModel {
	return {
		id: Number(replay.id),
		whiteId: Number(replay.whiteId),
		blackId: Number(replay.blackId),
		whiteName: replay.whiteName,
		blackName: replay.blackName,
		whiteCountry: replay.whiteCountry,
		blackCountry: replay.blackCountry,
		whiteElo: replay.whiteElo,
		blackElo: replay.blackElo,
		playedOn: replay.playedOn,
		result: replay.result,
		cause: replay.cause,
		mode: replay.mode,
		whiteEloDiff: replay.whiteEloDiff,
		blackEloDiff: replay.blackEloDiff,
		winEloDiff: replay.winEloDiff,
		loseEloDiff: replay.loseEloDiff
	};
}


export interface EloHistory {
	timestamp: string;
	elo: number;
}

export type EloBuckets = EloHistory[];


export interface FullPlayerModel {
	user: UserModel;
	replayList: ReplayModel[];
	stats: UserStatsEntity;
}

export interface PlayerModel {
	id: number;
	name: string;
	country: string;
	elo: number;
	isGuest: boolean;
	present: boolean;
}

export interface Hex {
	file: number;
	rank: number;
}

export interface ChessModel {
	id: string;
	whitePlayer: PlayerModel;
	blackPlayer: PlayerModel;
	firstColor: string;
	mode: string;
	ended: boolean;
}

export interface Chat {
	id: string;
    player?: PlayerState;
    message: string;
    sentAt: Date;
}

export function mapChatMessage(chat: ChatMessage): Chat {
	return {
		id: chat.id,
		player: chat.player,
		message: chat.message,
		sentAt: new Date(chat.sentAt),
	};
}

export interface ServiceModel {
	status: number;
	message?: string;
	error: string;
	errors: Record<string, string>
}
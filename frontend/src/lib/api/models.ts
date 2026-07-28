import type { ChatMessage, PlayerState } from '$lib/pb/messages';
import { nameMapIntoOptions } from './mapper';

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

export interface Session {
	id: number;
	username: string;
	country: string;
	elo: number;
	ttlSecs: number | null;
}

export type User = {
	id: number;
	username: string;
	country: string;
	rank: number;
	bio: string;
	joinedOn: string;
}

export function isGuestUser(user: PlayerState | Player) {
	return user.id < 0;
}

export type LeaderboardUser = User & {
	elo: number;
	highestElo: number;
	wins: number;
	losses: number;
	draws: number;
	winrate: number;
}

export interface UserStats {
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

export interface Challenge {
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

export interface Replay {
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

export interface EloHistory {
	timestamp: string;
	elo: number;
}

export type EloBuckets = EloHistory[];


export interface FullPlayer {
	user: User;
	replayList: Replay[];
	stats: UserStats;
}

export interface Player {
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

export interface ChessMetadata {
	gameId: string;
	whitePlayer: Player;
	blackPlayer: Player;
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

export interface ServiceResponse {
	status: number;
	message?: string;
	error: string;
	errors: Record<string, string>
}
import type { PlayerState } from '$lib/pb/messages';

export type Action = 'delete' | 'reject' | 'accept';

export type ColorSelect = 'RANDOM' | 'WHITE' | 'BLACK';

export type GameMode =
	| "TIMED_1+0"
	| "TIMED_3+2"
	| "TIMED_15+10"
	| "CORRESPONDENCE_1"
	| "CORRESPONDENCE_7"
	| "CORRESPONDENCE_14";

export const GameModeNameMap: Record<string, string> = {
	"TIMED_1+0": "Bullet",
	"TIMED_3+2": "Blitz",
	"TIMED_15+10": "Rapid",
	"CORRESPONDENCE_1": "Correspondence 1d",
	"CORRESPONDENCE_7": "Correspondence 7d",
	"CORRESPONDENCE_14": "Correspondence 14d",
};

export const TimedGameModes = ["TIMED_1+0", "TIMED_3+2", "TIMED_15+10"];

export interface SessionModel {
	id: number;
	username: string;
	country: string;
	elo: number;
	ttlSecs: number | null;
}

export interface UserModel {
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
}

export interface EloHistory {
	timestamp: string;
	elo: number;
}

export type EloBuckets = EloHistory[];


export interface FullUserModel {
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

export interface ServiceModel {
	status: number;
	message?: string;
	errors: string | Record<string, string>
}
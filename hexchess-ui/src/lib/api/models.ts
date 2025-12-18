export type Action = 'delete' | 'reject' | 'accept';

export type ColorSelect = 'RANDOM' | 'WHITE' | 'BLACK';

export type TimeControl = 'UNLIMITED' | 'REAL_TIME' | 'CORRESPONDENCE';

export type Timeframe = "1m" | "3m" | "6m" | "1y" | "all";

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
	elo: number;
	highestElo: number;
	wins: number;
	losses: number;
	rank: number;
	bio: string;
	joinedOn: string;
	total: number;
	winRate: number;
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
	timeControl: string;
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
	cause: number;
	whiteEloDiff: number;
	blackEloDiff: number;
}

export interface EloHistory {
	timestamp: string;
	elo: number;
}

export type EloBuckets = EloHistory[];

export const ReplayModeMap: Record<string, string> = {
	'ALL': "ALL Modes",
	'CORRESPONDENCE': "Correspondence",
	'REAL_TIME': "RealTime",
	'UNLIMITED': 'Unlimited'
}

export interface FullUserModel {
	user: UserModel;
	replayList: ReplayModel[];
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
	timeControl: string;
	ended: boolean;
}

export interface ServiceModel {
	status: number;
	message: string;
}
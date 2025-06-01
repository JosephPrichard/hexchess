export type Action = 'delete' | 'reject' | 'accept';

export type ReplayResult = 'DRAW' | 'WHITE_WIN' | 'BLACK_WIN';

export type ReplayCause = 'CHECKMATE' | 'FORFEIT';

export type ColorSelect = 'RANDOM' | 'WHITE' | 'BLACK';

export type TimeControl = 'UNLIMITED' | 'REAL_TIME' | 'CORRESPONDENCE';

export interface Session {
	id: number;
	username: string;
	country: string;
	elo: number;
	ttlSecs?: number;
}

export interface User {
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

export interface Challenge {
	challengerId: number;
	challengerName: string;
	challengerCountry: string;
	challengerElo: number;
	challengeeId: number;
	challengeeName: string;
	challengeeCountry: string;
	challengeeElo: number;
	timeControl: string;
	madeAgo: string;
	expiresIn: string;
}

export interface Replay {
	id: number;
	whiteId: number;
	blackId: number;
	whiteName: string;
	blackName: string;
	whiteCountry: string;
	blackCountry: string;
	winElo: number;
	loseElo: number;
	whiteElo: number;
	blackElo: number;
	playedOn: string;
	result: ReplayResult;
	cause: ReplayCause;
	whiteEloDiff: number;
	blackEloDiff: number;
}

export interface UserWithReplays {
	user: User;
	replayList: Replay[];
}
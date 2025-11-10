export type Action = 'delete' | 'reject' | 'accept';

export type ColorSelect = 'RANDOM' | 'WHITE' | 'BLACK';

export type TimeControl = 'UNLIMITED' | 'REAL_TIME' | 'CORRESPONDENCE';

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
	madeAgo: string;
	expiresIn: string;
}

export interface ReplayModel {
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
	result: number;
	cause: number;
	whiteEloDiff: number;
	blackEloDiff: number;
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
}

export interface Hexagon {
	file: number;
	rank: number;
}

export interface ChessModel {
	id: string,
	whitePlayer: PlayerModel | null,
	blackPlayer: PlayerModel | null,
	firstColor: number,
	timeControl: number,
	ended: boolean
}

export interface ServiceModel {
	status: number;
	message: string;
}

export type Action = 'delete' | 'reject' | 'accept';

export type ReplayResult = 'DRAW' | 'WHITE_WIN' | 'BLACK_WIN';

export type ReplayCause = 'CHECKMATE' | 'FORFEIT';

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
	result: ReplayResult;
	cause: ReplayCause;
	whiteEloDiff: number;
	blackEloDiff: number;
}

export interface UserWithReplaysModel {
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

export interface PieceMoveModel {
	piece: number;
	from: {
		file: number;
		rank: number;
	};
	to: {
		file: number;
		rank: number;
	};
}

export type MoveListModel = PieceMoveModel[];

export interface ChessModel {
	id: string,
	whitePlayer: PlayerModel | null,
	blackPlayer: PlayerModel | null,
	firstColor: ColorSelect,
	timeControl: TimeControl,
	ended: boolean
}

export interface ServiceModel {
	status: number;
	message: string;
}

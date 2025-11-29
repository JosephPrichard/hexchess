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
	result: string;
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

export interface Hex {
	file: number;
	rank: number;
}

export interface ChessModel {
	id: string,
	whitePlayer: PlayerModel | null,
	blackPlayer: PlayerModel | null,
	firstColor: string,
	timeControl: string,
	ended: boolean
}

export interface ServiceModel {
	status: number;
	message: string;
}

export function formatReplayResult(result: string) {
	switch (result) {
		case 'WHITE_WINS':
			return 'White Victory';
		case 'BLACK_WINS':
			return 'Black Victory';
		case 'DRAW':
			return 'Draw';
		default:
			console.error('Unknown replay result', result);
			return '';
	}
}

export function formatTimeControl(timeControl: string) {
	switch (timeControl) {
		case "UNLIMITED":
			return 'Unlimited ∞+0';
		case "CORRESPONDENCE":
			return `Correspondence ${10}+${1}`;
		case "REAL_TIME":
			return `Realtime ${5}+${3}`;
		default:
			console.error('Unknown time control: ' + timeControl);
			return '-';
	}
}

export function formatElo(elo: number) {
	return (elo >= 0 ? '+' : '') + elo;
}

export function formatTimer(ms: number) {
	const minutes = String(Math.floor(ms / 60000)).padStart(2, '0');
	ms %= 60000;
	const seconds = String(Math.floor(ms / 1000)).padStart(2, '0');
	ms %= 1000;
	return `${minutes}:${seconds}:${String(ms).padStart(2, '0')}`;
}

export function formatTimestamp(ts: string): string {
	const timestamp = new Date(ts);
	const mm = String(timestamp.getMonth() + 1).padStart(2, "0");
	const dd = String(timestamp.getDate()).padStart(2, "0");
	const yyyy = timestamp.getFullYear();
	return `${mm}/${dd}/${yyyy}`;
}

export function formatJoinedOn(ts: string): string {
	return formatTimestamp(ts)
}

export function formatPlayedOn(ts: string): string {
	const timestamp = new Date(ts);

	const now = Date.now();
	const then = timestamp.getTime();
	const diffMs = now - then;

	const diffMinutes = Math.floor(diffMs / (1000 * 60));
	const diffHours = Math.floor(diffMinutes / 60);
	const diffDays = Math.floor(diffHours / 24);

	if (diffDays > 0) {
		return timestamp.toLocaleString();
	} else if (diffHours > 1) {
		return `${diffHours} hours ago`;
	} else if (diffHours === 1) {
		return "1 hour ago";
	} else if (diffMinutes === 1) {
		return "1 minute ago";
	} else {
		return `${diffMinutes} minutes ago`;
	}
}
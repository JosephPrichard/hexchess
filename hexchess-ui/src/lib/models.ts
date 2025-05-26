export const Piece = {
	empty: 0,
	whitePawn: 1,
	blackPawn: 2,
	whiteKnight: 3,
	blackKnight: 4,
	whiteBishop: 5,
	blackBishop: 6,
	whiteRook: 7,
	blackRook: 8,
	whiteQueen: 9,
	blackQueen: 10,
	whiteKing: 11,
	blackKing: 12
} as const;

export type PieceType = (typeof Piece)[keyof typeof Piece];

export const piecenames: Record<PieceType, string> = {
	[Piece.empty]: 'empty',
	[Piece.whitePawn]: 'white-pawn',
	[Piece.blackPawn]: 'black-pawn',
	[Piece.whiteKnight]: 'white-knight',
	[Piece.blackKnight]: 'black-knight',
	[Piece.whiteBishop]: 'white-bishop',
	[Piece.blackBishop]: 'black-bishop',
	[Piece.whiteRook]: 'white-rook',
	[Piece.blackRook]: 'black-rook',
	[Piece.whiteQueen]: 'white-queen',
	[Piece.blackQueen]: 'black-queen',
	[Piece.whiteKing]: 'white-king',
	[Piece.blackKing]: 'black-king'
};

export type Action = 'delete' | 'reject' | 'accept';

export type ReplayResult = 'DRAW' | 'WHITE_WIN' | 'BLACK_WIN';

export type ReplayCause = 'CHECKMATE' | 'FORFEIT';

export type ColorSelect = 'RANDOM' | 'WHITE' | 'BLACK';

export type TimeControl = 'UNLIMITED' | 'REAL_TIME' | 'CORRESPONDENCE';

export interface ChessBoard {
	isWhiteTurn: boolean;
	pieces: PieceType[][];
}

export interface ChessGame {
	board: ChessBoard;
	whiteMoves: PieceMoves[];
	blackMoves: PieceMoves[];
	takenWhitePieces: PieceType[];
	takenBlackPieces: PieceType[];
}

export interface Hexagon {
	file: number;
	rank: number;
}

export interface PieceMove {
	piece: PieceType;
	from: Hexagon;
	to: Hexagon;
}

export interface PieceMoves {
	hex: Hexagon;
	moves: Hexagon[];
}

export interface SessionView {
	id: number;
	username: string;
	country: string;
	elo: number;
	ttlSecs?: number;
}

export interface UserView {
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

export interface ChallengeView {
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

export interface ReplayView {
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

export interface UserWithReplaysView {
	user: UserView;
	replayList: ReplayView[];
}

export interface PlayerView {
	id: number;
	name: string;
	country?: string;
	elo?: number;
	isGuest: boolean;
}

export interface ChallengeMsg {
	challengerId: number;
	challengerName: string;
	challengerCountry: string;
	challengeeId: number;
	challengeeName: string;
	challengeeCountry: string;
}

export interface ErrorMsg {
	type: 'ERROR';
	message: string;
}

export interface ForfeitMsg {
	type: 'FORFEIT';
}

export interface JoinMsg {
	type: 'JOIN';
	blackPlayer: PlayerView;
	whitePlayer: PlayerView;
	joiningPlayer: PlayerView;
}

export interface SettingsMsg {
	type: 'SETTINGS';
	settings: GameSettings;
}

export interface StartMsg {
	type: 'START';
	room: ChessRoom;
	selfPlayer: PlayerView;
}

export interface MoveMsg {
	type: 'MOVE';
	move: PieceMove;
	game: ChessGame;
}

export interface ChatMsg {
	type: 'CHAT';
	player: PlayerView;
	message: string;
}

export type GameOutputMsg =
	| ErrorMsg
	| ForfeitMsg
	| StartMsg
	| JoinMsg
	| MoveMsg
	| ChatMsg;

export interface ChessRoom {
	id: number;
	game: ChessGame;
	moveList: PieceMove[];
	whitePlayer: PlayerView | null;
	blackPlayer: PlayerView | null;
	timeControl: TimeControl;
	isEnded: boolean;
}

export interface GameSettings {
	timeControl: TimeControl;
}
import type { ChessBoard } from '$lib/api/messages';

export const hexHeight = 62;
export const hexWidth = hexHeight * 1.2;
export const verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5];
export const ranksPerFile = [6, 7, 8, 9, 10, 11, 10, 9, 8, 7, 6];
export const colors = ['rgb(255, 207, 159)', 'rgb(233, 172, 112)', 'rgb(210,140,69)'];
export const selectedColor = 'rgba(148, 100, 148, 0.5)';
export const colorsOffset = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0];
export const chessRowHeight = 45;
export const maxChessRows = 12;

export const pieces = {
	empty: 0,

	whitePawn: 1,
	whiteKnight: 3,
	whiteBishop: 5,
	whiteRook: 7,
	whiteQueen: 9,
	whiteKing: 11,

	blackPawn: 2,
	blackKnight: 4,
	blackBishop: 6,
	blackRook: 8,
	blackQueen: 10,
	blackKing: 12,
}

export const whitePieces = [
	pieces.whitePawn, pieces.whiteKnight, pieces.whiteBishop,
	pieces.whiteRook, pieces.whiteQueen, pieces.whiteKing
];

export const blackPieces = [
	pieces.blackPawn, pieces.blackKnight, pieces.blackBishop,
	pieces.blackRook, pieces.blackQueen, pieces.blackKing
];

export const piecenames: Record<number, string> = {
	[pieces.empty]: 'empty',
	[pieces.whitePawn]: 'white-pawn',
	[pieces.blackPawn]: 'black-pawn',
	[pieces.whiteKnight]: 'white-knight',
	[pieces.blackKnight]: 'black-knight',
	[pieces.whiteBishop]: 'white-bishop',
	[pieces.blackBishop]: 'black-bishop',
	[pieces.whiteRook]: 'white-rook',
	[pieces.blackRook]: 'black-rook',
	[pieces.whiteQueen]: 'white-queen',
	[pieces.blackQueen]: 'black-queen',
	[pieces.whiteKing]: 'white-king',
	[pieces.blackKing]: 'black-king'
};

export const defaultBoard: ChessBoard = {
	isWhiteTurn: false,
	file: ranksPerFile.map(ranks => ({ pieces: Array(ranks).fill(0) }))
};

export type BoardErr = keyof typeof boardErrMessages;

export const boardErrMessages = {
	"ERR_PIECES": 'A board must have at least one piece on it.',
	"ERR_KINGS": 'A board must have a king for at each side.',
}
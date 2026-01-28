import type { ChessBoard, ChessGame, PieceMoves } from '../pb/messages';
import type { Hex } from '../api/models';

export const ranksPerFile = [6, 7, 8, 9, 10, 11, 10, 9, 8, 7, 6];

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

export const promotions = {
	queen: 1,
	rook: 2,
	bishop: 3,
	knight: 4,
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

export const piecepoints: Record<number, number> = {
	[pieces.whitePawn]: 1,
	[pieces.whiteKnight]: 3,
	[pieces.whiteBishop]: 3,
	[pieces.whiteRook]: 5,
	[pieces.whiteQueen]: 9,
};

export const defaultBoard: ChessBoard = {
	isWhiteTurn: true,
	file: ranksPerFile.map(ranks => ({ pieces: Array(ranks).fill(0) }))
};

export function isInBounds(hex: Hex) {
	return hex.file >= 0 && hex.file < ranksPerFile.length && hex.rank >= 0 && hex.rank < ranksPerFile[hex.file];
}

export function isPromotion(game: ChessGame | undefined, from: Hex, to: Hex) {
	const piece = game?.board?.file[from.file].pieces[from.rank];
	const isPawn = piece === pieces.whitePawn || piece === pieces.blackPawn;
	if (!isPawn) return false;

	const isWhiteTurn = game?.board?.isWhiteTurn;
	return isWhiteTurn ? to.rank === ranksPerFile[to.file] - 1 : to.rank === 0;
}

export const isPieceWhite = (piece: number) => piece % 2 === 1;

export const makePieceWhite = (piece: number) => piece - ((piece + 1) % 2);

export const deserializeHex = (h: bigint) => ({
	file: Number(h & 0xFFFFFFFFn),
	rank: Number((h >> 32n) & 0xFFFFFFFFn),
});

export const deserializeHexList = (hexagonList?: bigint[]) => hexagonList?.map(deserializeHex) ?? [];

export const getPiecePoints = (piece: number) => piecepoints[makePieceWhite(piece)] ?? [];

export const hexEq = (hex1?: { file?: number; rank?: number }, hex2?: { file?: number; rank?: number }) =>
	hex1?.file === hex2?.file && hex1?.rank === hex2?.rank;

export const makeMoveKey = (file: number, rank: number) => file + "," + rank;

export function isValidMove(game: ChessGame | undefined, {from, to}: {from: Hex, to: Hex}, asWhite: boolean) {
	const pms = (asWhite ? game?.whiteMoves : game?.blackMoves) ?? [];
	let moveIdx = pms.findIndex((pm) => hexEq({file: pm.fromFile, rank: pm.fromRank}, from));
	if (moveIdx < 0) {
		return false;
	}
	const moves = deserializeHexList(pms[moveIdx].moves);
	moveIdx = moves.findIndex((h) => hexEq(h, to));
	return moveIdx !== -1;
}

export function moveBoardPiece(game: ChessGame | undefined, from: Hex, to: Hex) {
	const board = game?.board;

	const isSameMove = from.file === to.file && from.rank === to.rank;
	if (!board || !isInBounds(to) || isSameMove) return game;

	const piece = board.file[from.file].pieces[from.rank];
	board.file[from.file].pieces[from.rank] = pieces.empty;
	board.file[to.file].pieces[to.rank] = piece;

	return { ...(game || defaultGame), board: board || defaultBoard };
}

export function clearBoard(game: ChessGame | undefined) {
	const board = game?.board;
	if (!board) return game;

	for (const file of board.file) {
		file.pieces.fill(pieces.empty);
	}
	return makeGame(board);
}

export function removeBoardPiece(game: ChessGame | undefined, hex: Hex) {
	const board = game?.board;
	if (!board || !isInBounds(hex)) return game;

	board.file[hex.file].pieces[hex.rank] = pieces.empty;
	return { ...(game || defaultGame), board: board || defaultBoard };
}

export function placeBoardPiece(game: ChessGame | undefined, hex: Hex, piece: number) {
	const board = game?.board;
	if (!board || !isInBounds(hex)) return game;

	board.file[hex.file].pieces[hex.rank] = piece;
	return { ...(game || defaultGame), board: board || defaultBoard };
}

export function setBoardTurn(game: ChessGame | undefined, turn: boolean) {
	const board = game?.board;
	if (!board) return board;

	board.isWhiteTurn = turn;
	return makeGame(board);
}

export function findPotentialMoves(game: ChessGame | undefined, hex: Hex): PieceMoves | undefined {
	if (!game) return;
	let potentialMoves: PieceMoves | undefined;

	let index = game.blackMoves.findIndex((move) => hex.file === move?.fromFile && hex.rank == move?.fromRank);
	if (index !== -1) {
		potentialMoves = game.blackMoves[index];
	}
	index = game.whiteMoves.findIndex((move) => hex.file === move?.fromFile && hex.rank == move?.fromRank);
	if (index !== -1) {
		potentialMoves = game.whiteMoves[index];
	}
	return potentialMoves;
}

export const defaultGame = makeGame(defaultBoard);

export function makeGame(board: ChessBoard | undefined): ChessGame {
	return {
		whiteMoves: [],
		blackMoves: [],
		takenWhitePieces: [],
		takenBlackPieces: [],
		moves: [],
		board: board
	}
}

export function iterBoard(board: ChessBoard, cb: (file: number, rank: number, piece: number) => void) {
	let file = 0;
	for (const bFile of board.file) {
		let rank = 0;
		for (const piece of bFile.pieces) {
			cb(file, rank, piece);
			rank++;
		}
		file++;
	}
}
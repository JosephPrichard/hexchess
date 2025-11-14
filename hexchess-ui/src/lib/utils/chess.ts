import type { ChessBoard, ChessGame, PieceMove, PieceMoves } from '$lib/api/messages';
import type { Hex } from '$lib/api/model';

const symbols = ['?', 'P', 'p', 'N', 'n', 'B', 'b', 'R', 'r', 'Q', 'q', 'K', 'k'];
const files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k'];

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

export const defaultGame = {
	whiteMoves: [],
	blackMoves: [],
	takenWhitePieces: [],
	takenBlackPieces: [],
	moveList: []
}

export const defaultBoard: ChessBoard = {
	isWhiteTurn: true,
	file: ranksPerFile.map(ranks => ({ pieces: Array(ranks).fill(0) }))
};

export function isWhite(piece: number) {
	return piece % 2 === 1;
}

export function isBlack(piece: number) {
	return !isWhite(piece);
}

export function stringOfMove(move: PieceMove) {
	const symbol = symbols[move.piece] || '?';
	const toFile = files[move.toFile];
	const toRank = String(move.toRank + 1);
	return symbol + toFile + toRank;
}

export function mapHexagon(hexagon: bigint) {
	return {
		file: Number(hexagon & 0xFFFFFFFFn),
		rank: Number((hexagon >> 32n) & 0xFFFFFFFFn)
	}
}

export function mapHexagonList(hexagonList?: bigint[]): Hex[] {
	return hexagonList?.map(mapHexagon) || [];
}

export interface Selection {
	potentialMoves: PieceMoves | undefined;
	hex: Hex | undefined;
}

export function handleSelectPiece(game: ChessGame, selection: Selection, next: Hex): Selection {
	let index = game.blackMoves.findIndex((move) =>
		next.file === move?.fromFile && next.rank == move?.fromRank);
	if (index !== -1) {
		const potentialMoves = game.blackMoves[index];
		return { potentialMoves: potentialMoves, hex: next };
	} else {
		index = game.whiteMoves.findIndex((move) =>
			next.file === move?.fromFile && next.rank == move?.fromRank);
		if (index !== -1) {
			const potentialMoves = game.whiteMoves[index];
			return { potentialMoves: potentialMoves, hex: next };
		}
	}
	return { potentialMoves: selection.potentialMoves, hex: undefined };
}

export function handleDeSelectPiece(): Selection {
	return { potentialMoves: undefined, hex: undefined };
}

export function countPieces(board?: ChessBoard) {
	if (!board) {
		return [0, 0];
	}
	let pieceCount = 0;
	let kingCount = 0;
	for (const file of board.file) {
		for (const piece of file.pieces) {
			if (piece != pieces.empty) {
				pieceCount++;
			}
			if (piece == pieces.blackKing || piece == pieces.whiteKing) {
				kingCount++;
			}
		}
	}
	return [pieceCount, kingCount]
}
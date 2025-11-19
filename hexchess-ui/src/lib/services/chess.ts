import type { ChessBoard, ChessGame, PieceMove, PieceMoves } from '../api/messages';
import type { Hex } from '../api/model';

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

export const NoSelection: Selection = { potentialMoves: undefined, hex: undefined };

export function getNewSelection(game: ChessGame | undefined, hex: Hex): Selection {
	if (!game)
		return NoSelection;

	let potentialMoves: PieceMoves | undefined;

	let index = game.blackMoves.findIndex((move) => hex.file === move?.fromFile && hex.rank == move?.fromRank);
	if (index !== -1) {
		potentialMoves = game.blackMoves[index];
	}
	index = game.whiteMoves.findIndex((move) => hex.file === move?.fromFile && hex.rank == move?.fromRank);
	if (index !== -1) {
		potentialMoves = game.whiteMoves[index];
	}

	return { potentialMoves: potentialMoves, hex: hex};
}

export function isMoveValid(game: ChessGame | undefined, from: Hex, to: Hex) {
	const currMoves =
		(game?.board?.isWhiteTurn ? game.whiteMoves : game?.blackMoves)
		|| [];
	const pieceMoves = currMoves.find((hex) =>
		hex.fromFile == from.file && hex.fromRank == from.rank)?.moves || [];
	const index = pieceMoves.findIndex((hex) => {
		const mh = mapHexagon(hex);
		return mh.file == to.file && mh.rank == to.rank
	});
	return index >= 0;
}

export const defaultGame: ChessGame = {
	whiteMoves: [],
	blackMoves: [],
	takenWhitePieces: [],
	takenBlackPieces: [],
	moveList: [],
	board: defaultBoard,
};

export function newGame(board: ChessBoard | undefined) {
	return { ...defaultGame, board: board }
}

export function newMove(from: Hex, to: Hex) {
	return { piece: 0, fromFile: from.file, fromRank: from.rank, toFile: to.file, toRank: to.rank };
}

export function moveBoardPiece(board: ChessBoard | undefined, from: Hex, to: Hex) {
	if (board) {
		const piece = board.file[from.file].pieces[from.rank];
		board.file[from.file].pieces[from.rank] = pieces.empty;
		board.file[to.file].pieces[to.rank] = piece;
	}
	return newGame(board);
}

export function clearBoard(board: ChessBoard | undefined) {
	if (board) {
		for (const file of board.file) {
			file.pieces.fill(pieces.empty);
		}
	}
	return newGame(board);
}

export function removeBoardPiece(board: ChessBoard | undefined, hex: Hex) {
	if (board) {
		board.file[hex.file].pieces[hex.rank] = pieces.empty;
	}
	return newGame(board);
}

export function placeBoardPiece(board: ChessBoard | undefined, hex: Hex, piece: number) {
	if (board) {
		board.file[hex.file].pieces[hex.rank] = piece;
	}
	return newGame(board);
}

export function setBoardTurn(board: ChessBoard | undefined, turn: boolean) {
	if (board) {
		board.isWhiteTurn = turn;
	}
	return newGame(board);
}
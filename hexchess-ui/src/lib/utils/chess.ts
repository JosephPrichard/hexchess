import type { ChessBoard, ChessGame, PieceMove, PieceMoves } from '../api/messages';
import type { Hex } from '../api/model';
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
	rook: 3,
	bishop: 5,
	knight: 7,
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

export function isPieceWhite(piece: number) {
	return piece % 2 === 1;
}

export function isPieceBlack(piece: number) {
	return !isPieceWhite(piece);
}

export function mapHexagons(hexagonList?: bigint[]): Hex[] {
	return hexagonList?.map(h => ({ file: Number(h & 0xFFFFFFFFn), rank: Number((h >> 32n) & 0xFFFFFFFFn) })) || [];
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

function isInBounds(hex: Hex) {
	return hex.file >= 0 && hex.file < ranksPerFile.length && hex.rank >= 0 && hex.rank < ranksPerFile[hex.file];
}

export function moveBoardPiece(board: ChessBoard | undefined, from: Hex, to: Hex) {
	if (board &&
		(isInBounds(to) && (from.file != to.file || from.rank != to.rank))
	) {
		const piece = board.file[from.file].pieces[from.rank];
		board.file[from.file].pieces[from.rank] = pieces.empty;
		board.file[to.file].pieces[to.rank] = piece;
		return makeGame(board);
	}
	return undefined;
}

export function clearBoard(board: ChessBoard | undefined) {
	if (board) {
		for (const file of board.file) {
			file.pieces.fill(pieces.empty);
		}
	}
	return makeGame(board);
}

export function removeBoardPiece(board: ChessBoard | undefined, hex: Hex) {
	if (board) {
		board.file[hex.file].pieces[hex.rank] = pieces.empty;
	}
	return makeGame(board);
}

export function placeBoardPiece(board: ChessBoard | undefined, hex: Hex, piece: number) {
	if (isInBounds(hex) && board) {
		board.file[hex.file].pieces[hex.rank] = piece;
		return makeGame(board);
	}
	return undefined;
}

export function setBoardTurn(board: ChessBoard | undefined, turn: boolean) {
	if (board) {
		board.isWhiteTurn = turn;
		return makeGame(board);
	}
	return undefined;
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

let GlobalPieceKey = 0;

export type PlacedPiece = {piece: number, file: number, rank: number};

export function findKeyedPieces(boardState: ChessBoard, prevPieces?: [number, PlacedPiece][]) {
	const nextPieces: Map<number, PlacedPiece> = new Map();
	if (prevPieces !== undefined) {
		const table: Map<number, {key: number, file: number, rank: number}[]> = new Map();
		for (const [key, value] of prevPieces) {
			const arr = table.get(value.piece);
			const record = {key, file: value.file, rank: value.rank};
			if (arr === undefined) {
				table.set(value.piece, [record])
			} else {
				arr.push(record);
			}
		}
		iterBoard(boardState, (file, rank, piece) => {
			if (piece === pieces.empty) {
				return;
			}
			const arr = table.get(piece);
			if (arr === undefined) {
				nextPieces.set(GlobalPieceKey++, {file, rank, piece});
				return;
			}
			const recordIdx = arr.findIndex(r => r.file === file && r.rank === rank);
			if (recordIdx !== -1) {
				nextPieces.set(arr[recordIdx].key, {file, rank, piece});
				arr.splice(recordIdx, 1);
				return;
			}
			const record = arr.pop()
			if (record !== undefined) {
				nextPieces.set(record.key, {file, rank, piece});
				return;
			}
			nextPieces.set(GlobalPieceKey++, {file, rank, piece});
		})
	} else {
		iterBoard(boardState, (file, rank, piece) => {
			if (piece !== pieces.empty) {
				nextPieces.set(GlobalPieceKey++, {file, rank, piece});
			}
		});
	}
	return nextPieces.entries().toArray();
}
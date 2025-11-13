import type { ChessBoard, ChessGame, PieceMove, PieceMoves } from '$lib/api/messages';
import type { Hexagon } from '$lib/api/model';
import { pieces } from '$lib/utils/globals';

const symbols = ['?', 'P', 'p', 'N', 'n', 'B', 'b', 'R', 'r', 'Q', 'q', 'K', 'k'];
const files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k'];

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

export function mapHexagonList(hexagonList?: bigint[]): Hexagon[] {
	if (!hexagonList) {
		return [];
	}
	return hexagonList.map(hexagon => ({
		file: Number(hexagon & 0xFFFFFFFFn),
		rank: Number((hexagon >> 32n) & 0xFFFFFFFFn)
	}));
}

export interface Selection {
	potentialMoves: PieceMoves | undefined;
	hex: Hexagon | undefined;
}

export function handleSelectPiece(game: ChessGame, selection: Selection, next: Hexagon): Selection {
	let index = game.blackMoves.findIndex((move) => next.file === move?.fromFile && next.rank == move?.fromRank);
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
	const nextSelection = { potentialMoves: selection.potentialMoves, hex: undefined };
	console.log("New selection:", next, nextSelection);
	return nextSelection;
}

export function handleDeSelectPiece(): Selection {
	return { potentialMoves: undefined, hex: undefined };
}

export function countPieces(board?: ChessBoard) {
	let pieceCount = 0;
	let kingCount = 0;
	if (board) {
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
	}
	return [pieceCount, kingCount]
}
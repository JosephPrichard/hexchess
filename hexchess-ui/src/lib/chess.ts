import type { ChessBoard, PieceMove } from '$lib/messages';

const symbols = ['p', 'p', 'n', 'n', 'b', 'b', 'r', 'r', 'q', 'q', 'k', 'k'];
const files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

export function stringOfMove(move: PieceMove) {
	if (move.to === undefined || move.from === undefined) {
		throw new Error("Move 'to' and 'from' must be defined, got " + JSON.stringify(move));
	}

	const symbol = symbols[move.piece] || '?';
	const toFile = files[move.to.file];
	const toRank = String(move.to.rank + 1);
	return symbol + toFile + toRank;
}

export function translateBoard(index: number, moveList: PieceMove[], board: ChessBoard) {
	board = structuredClone(board);
	for (let i = 0; i <= index; i++) {
		const move = moveList[i];
		if (move.to === undefined || move.from === undefined) {
			throw new Error("Move 'to' and 'from' must be defined, got " + JSON.stringify(move));
		}

		const { to: { rank: toRank, file: toFile }, from: { rank: fromRank, file: fromFile } } = move;

		board.pieces[toFile][toRank] = board.pieces[fromFile][fromRank];
		board.pieces[fromFile][fromRank] = 0;
	}
	return board;
}

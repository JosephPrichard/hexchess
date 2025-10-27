import type { ChessBoard, PieceMove } from '$lib/api/messages';
import type { PieceMoveModel } from '$lib/api/model';

const symbols = ['?', 'P', 'p', 'N', 'n', 'B', 'b', 'R', 'r', 'Q', 'q', 'K', 'k'];
const files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k'];

export function stringOfMove(move: PieceMove | PieceMoveModel) {
	if (move.to === undefined || move.from === undefined) {
		throw new Error("Move 'to' and 'from' must be defined, got " + JSON.stringify(move));
	}

	const symbol = symbols[move.piece] || '?';
	const toFile = files[move.to.file];
	const toRank = String(move.to.rank + 1);
	const str = symbol + toFile + toRank;
	// console.log(move, str);
	return str;
}

export function translateBoard(index: number, moveList: PieceMove[], board: ChessBoard) {
	board = structuredClone(board);
	for (let i = 0; i <= index; i++) {
		const move = moveList[i];
		if (move.to === undefined || move.from === undefined) {
			throw new Error("Move 'to' and 'from' must be defined, got " + JSON.stringify(move));
		}
		const {
			to: { rank: toRank, file: toFile },
			from: { rank: fromRank, file: fromFile }
		} = move;

		board.file[toFile].pieces[toRank] = board.file[fromFile].pieces[fromRank];
		board.file[fromFile].pieces[fromRank] = 0;
	}
	return board;
}

import type { ChessRoom, Hexagon, PieceMoves } from '$lib/models';

export function isHexagonEqual(hex1: Hexagon | undefined, hex2: Hexagon | undefined) {
	return hex1?.file === hex2?.file && hex1?.rank === hex2?.rank;
}

export function findPotentialMoves(nextSelection: Hexagon, room: ChessRoom) {
	let index = room.game.blackMoves.findIndex(move => isHexagonEqual(move.hex, nextSelection));
	if (index !== -1) {
		return room.game.blackMoves[index];
	}
	index = room.game.whiteMoves.findIndex(move => isHexagonEqual(move.hex, nextSelection));
	if (index !== -1) {
		return room.game.whiteMoves[index];
	}
}
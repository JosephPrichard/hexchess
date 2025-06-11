import type { ChessGame, Hexagon, PieceMoves } from '$lib/api/messages';

export function onSelectPiece(game: ChessGame, newSelection: Hexagon, oldPotentialMoves: PieceMoves | undefined) {
	if (newSelection.file === newSelection.file && newSelection.rank == newSelection.rank) {
		return { potentialMoves: undefined, newSelection: undefined };
	}

	let index = game.blackMoves.findIndex((move) =>
		newSelection.file === move?.hex?.file && newSelection.rank == move?.hex?.rank);
	if (index !== -1) {
		const potentialMoves = game.blackMoves[index];
		return { potentialMoves: potentialMoves, newSelection };
	} else {
		index = game.whiteMoves.findIndex((move) =>
			newSelection.file === move?.hex?.file && newSelection.rank == move?.hex?.rank);
		if (index !== -1) {
			const potentialMoves = game.whiteMoves[index];
			return { potentialMoves: potentialMoves, newSelection };
		}
	}

	return { potentialMoves: oldPotentialMoves, newSelection: undefined };
}
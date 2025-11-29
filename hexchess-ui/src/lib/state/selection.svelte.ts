import type { Hex } from '$lib/api/models';
import type { ChessGame, PieceMoves } from '$lib/pb/messages';

export interface SelectionState {
	potentialMoves: PieceMoves | undefined;
	hex: Hex | undefined;
}

export const NoSelection: SelectionState = { potentialMoves: undefined, hex: undefined };

export function makeSelectionState() {
	let value: SelectionState = $state({
		potentialMoves: undefined,
		hex: undefined,
	});

	function select(game: ChessGame | undefined, hex: Hex) {
		if (!game) {
			return
		}
		let potentialMoves: PieceMoves | undefined;

		let index = game.blackMoves.findIndex((move) => hex.file === move?.fromFile && hex.rank == move?.fromRank);
		if (index !== -1) {
			potentialMoves = game.blackMoves[index];
		}
		index = game.whiteMoves.findIndex((move) => hex.file === move?.fromFile && hex.rank == move?.fromRank);
		if (index !== -1) {
			potentialMoves = game.whiteMoves[index];
		}

		value.potentialMoves = potentialMoves;
		value.hex = hex;
	}

	function deSelect() {
		value.potentialMoves = NoSelection.potentialMoves;
		value.hex = NoSelection.hex;
	}

	return { value, select, deSelect };
}
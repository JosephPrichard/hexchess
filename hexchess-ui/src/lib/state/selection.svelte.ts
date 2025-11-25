import type { ChessGame, PieceMoves } from '$lib/api/messages';
import type { Hex } from '$lib/api/model';

export interface Selection {
	potentialMoves: PieceMoves | undefined;
	hex: Hex | undefined;
}

export const NoSelection: Selection = { potentialMoves: undefined, hex: undefined };

export function makeMoveSelectionState() {
	let value: Selection = $state({
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
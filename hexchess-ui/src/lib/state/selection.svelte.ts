import type { Hex } from '$lib/api/models';
import type { ChessGame, PieceMoves } from '$lib/pb/messages';
import {chessService} from "$lib/service/chess";

export interface SelectionState {
	potentialMoves: PieceMoves | undefined;
	hex: Hex | undefined;
}

export const NoSelection: SelectionState = { potentialMoves: undefined, hex: undefined };

export function makeSelectionState() {
	let state: SelectionState = $state({
		potentialMoves: undefined,
		hex: undefined,
	});

	function select(game: ChessGame | undefined, hex: Hex) {
		state.potentialMoves = chessService.findPotentialMoves(game, hex);
		state.hex = hex;
	}

	function deSelect() {
		state.potentialMoves = NoSelection.potentialMoves;
		state.hex = NoSelection.hex;
	}

	function getPotentialMoves() {
		const pm = state.potentialMoves;
		return chessService.deserializeHexList(pm?.moves);
	}

	return { state, getPotentialMoves, select, deSelect };
}
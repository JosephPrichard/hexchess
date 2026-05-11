export interface MoveState {
	moveIndex: number | undefined;
	moveCount: number;
}

export function makeMoveState() {
	let state: MoveState = $state({
		moveIndex: undefined,
		moveCount: 0,
	})

	function selectMove(i: number) {
		state.moveIndex = i;
	}

	function goLeft() {
		if (state.moveIndex === undefined) {
			state.moveIndex = 0;
		} else if (state.moveIndex > 0) {
			state.moveIndex--;
		}
	}

	function goRight() {
		if (state.moveIndex === undefined) {
			state.moveIndex = 0;
		} else if (state.moveIndex < state.moveCount - 1) {
			state.moveIndex++;
		}
	}

	function updateMoveCount(count: number) {
		state.moveCount = count;
	}

	return { state, selectMove, goLeft, goRight, updateMoveCount }
}
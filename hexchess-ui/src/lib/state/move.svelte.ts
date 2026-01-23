export interface MoveState {
	moveIndex: number | undefined;
	isWhitePerspective: boolean;
	moveCount: number;
}

export function makeMoveState() {
	let state: MoveState = $state({
		moveIndex: undefined,
		isWhitePerspective: true,
		moveCount: 0,
	})

	function selectMove(i: number) {
		state.moveIndex = i;
	}

	function deSelectMove() {
		state.moveIndex = undefined;
	}

	function flip() {
		state.isWhitePerspective = !state.isWhitePerspective;
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

	return { state, selectMove, deSelectMove, flip, goLeft, goRight, updateMoveCount }
}
interface MoveState {
	moveIndex: number | undefined,
	isWhitePerspective: boolean,
	moveCount: number
}

export function createMoveState() {
	let value: MoveState = $state({
		moveIndex: undefined,
		isWhitePerspective: true,
		moveCount: 0
	})

	function selectMove(i: number) {
		value.moveIndex = i;
	}

	function flip() {
		value.isWhitePerspective = !value.isWhitePerspective;
	}

	function goLeft() {
		if (value.moveIndex === undefined) {
			value.moveIndex = 0;
		} else if (value.moveIndex > 0) {
			value.moveIndex--;
		}
	}

	function goRight() {
		if (value.moveIndex === undefined) {
			value.moveIndex = 0;
		} else if (value.moveIndex < value.moveCount - 1) {
			value.moveIndex++;
		}
	}

	function updateMoveCount(count: number) {
		value.moveCount = count;
	}

	return { value, selectMove, flip, goLeft, goRight, updateMoveCount}
}
function renderReplay() {
    const boardElem = document.getElementById("chess-board");
    const moveListElem = document.getElementById("replay-move-list");
    const flipButton = document.getElementById("flip-board-button");
    const rightButton = document.getElementById("right-button");
    const leftButton = document.getElementById("left-button");

    const moveList = JSON.parse(document.getElementById("move-list-data").innerHTML);
    const initialBoard = JSON.parse(document.getElementById("initial-board-data").innerHTML);

    const boardCache = [];
    let state = {
        currentIndex: undefined,
        isFlipped: false,
        selectMoveElem: undefined,
    };

    function getBoard(index) {
        if (boardCache[index] !== undefined) {
            return boardCache[index];
        }
        const board = structuredClone(initialBoard);
        for (let i = 0; i < index; i++) {
            const {to: {rank: toRank, file: toFile}, from: {rank: fromRank, file: fromFile}} = moveList[i];
            board.pieces[toFile][toRank] = board.pieces[fromFile][fromRank]
            board.pieces[fromFile][fromRank] = EMPTY;
        }
        boardCache[index] = board;
        return board;
    }

    function handleOnMove(index, moveElem) {
        const board = getBoard(index);
        state.currentIndex = index;
        renderBoard(boardElem, board, state.isFlipped);

        if (state.selectMoveElem) {
            state.selectMoveElem.classList.remove("selected-move");
        }
        moveElem.classList.add("selected-move");
        state.selectMoveElem = moveElem;
    }

    renderBoard(boardElem, initialBoard, state.isFlipped);
    renderMoveList(moveListElem, moveList, handleOnMove);

    flipButton.addEventListener('click', () => {
        state.isFlipped = !state.isFlipped;
        const board = state.currentIndex !== undefined ? getBoard(state.currentIndex) : initialBoard;
        renderBoard(boardElem, board, state.isFlipped);
    });
    leftButton.addEventListener('click', () => {
        if (state.currentIndex === undefined) {
            state.currentIndex = 0;
        } else {
            if (state.currentIndex === 0) {
                return;
            }
            state.currentIndex--;
        }
        handleOnMove(state.currentInde, document.getElementById(`move-${state.currentIndex}`));
    });
    rightButton.addEventListener('click', () => {
        if (state.currentIndex === undefined) {
            state.currentIndex = 0;
        } else {
            if (state.currentIndex === moveList.length - 1) {
                return;
            }
            state.currentIndex++;
        }
        handleOnMove(state.currentIndex, document.getElementById(`move-${state.currentIndex}`));
    });
}
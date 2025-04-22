async function getInitialBoard() {

}

async function renderAndAttachEventListeners() {
    let state = {
        boardCache: [],
        currentIndex: undefined,
        isFlipped: false,
        selectMoveElem: undefined,
    };

    const resp = await fetch("/static/initial-board", { method: "GET" });
    const initialBoard = await resp.json();

    const moveList = JSON.parse(document.getElementById("move-list-data").innerHTML);

    const boardElem = document.getElementById("chess-board");
    const moveListElem = document.getElementById("replay-move-list");
    const flipButton = document.getElementById("flip-board-button");
    const rightButton = document.getElementById("right-button");
    const leftButton = document.getElementById("left-button");

    function getBoard(index) {
        if (state.boardCache[index] !== undefined) {
            return state.boardCache[index];
        }
        const board = structuredClone(initialBoard);
        for (let i = 0; i < index + 1; i++) {
            const {
                to: {
                    rank: toRank,
                    file: toFile
                },
                from: {
                    rank: fromRank,
                    file: fromFile
                }
            } = moveList[i];
            board.pieces[toFile][toRank] = board.pieces[fromFile][fromRank];
            board.pieces[fromFile][fromRank] = EMPTY;
        }
        state.boardCache[index] = board;
        return board;
    }

    function handleOnMove(index, moveElem) {
        const board = getBoard(index);
        state.currentIndex = index;
        renderBoard(boardElem, board, state.isFlipped);

        if (state.selectMoveElem) {
            state.selectMoveElem.classList.remove("selected-move");
        }
        state.selectMoveElem = moveElem;
        moveElem.classList.add("selected-move");
    }

    function onFlipClick() {
        state.isFlipped = !state.isFlipped;
        const board = state.currentIndex !== undefined ? getBoard(state.currentIndex) : initialBoard;
        renderBoard(boardElem, board, state.isFlipped);
    }

    function onLeftClick() {
        if (state.currentIndex === undefined) {
            state.currentIndex = 0;
        } else if (state.currentIndex > 0) {
            state.currentIndex--;
        } else {
            return;
        }
        const elem = document.getElementById(`move-${state.currentIndex}`);
        handleOnMove(state.currentIndex, elem);
    }

    function onRightClick() {
        if (state.currentIndex === undefined) {
            state.currentIndex = 0;
        } else if (state.currentIndex < moveList.length - 1) {
            state.currentIndex++;
        } else {
            return;
        }
        const elem = document.getElementById(`move-${state.currentIndex}`);
        handleOnMove(state.currentIndex, elem);
    }

    flipButton.addEventListener('click', onFlipClick);
    leftButton.addEventListener('click', onLeftClick);
    rightButton.addEventListener('click', onRightClick);

    renderBoard(boardElem, initialBoard, state.isFlipped);
    renderMoveList(moveListElem, moveList, handleOnMove);
}

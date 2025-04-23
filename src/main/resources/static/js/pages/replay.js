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

    const boardElement = document.getElementById("chess-board");
    const moveListElement = document.getElementById("replay-move-list");

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
        renderBoard(boardElement, board, state.isFlipped);

        if (state.selectMoveElem) {
            state.selectMoveElem.classList.remove("selected-move");
        }
        state.selectMoveElem = moveElem;
        moveElem.classList.add("selected-move");
    }

    function onFlipClick() {
        state.isFlipped = !state.isFlipped;
        const board = state.currentIndex !== undefined ? getBoard(state.currentIndex) : initialBoard;
        renderBoard(boardElement, board, state.isFlipped);
    }

    function onLeftClick() {
        if (state.currentIndex === undefined) {
            state.currentIndex = 0;
        } else if (state.currentIndex > 0) {
            state.currentIndex--;
        } else {
            return;
        }
        handleOnMove(state.currentIndex, document.getElementById(`move-${state.currentIndex}`));
    }

    function onRightClick() {
        if (state.currentIndex === undefined) {
            state.currentIndex = 0;
        } else if (state.currentIndex < moveList.length - 1) {
            state.currentIndex++;
        } else {
            return;
        }
        handleOnMove(state.currentIndex, document.getElementById(`move-${state.currentIndex}`));
    }

    document.getElementById("flip-board-button").addEventListener('click', onFlipClick);
    document.getElementById("right-button").addEventListener('click', onLeftClick);
    document.getElementById("left-button").addEventListener('click', onRightClick);

    renderBoard(boardElement, initialBoard, state.isFlipped);
    renderMoveList(moveListElement, moveList, handleOnMove);
}

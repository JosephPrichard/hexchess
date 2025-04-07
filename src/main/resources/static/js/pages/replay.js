function handleReplayEvents() {
    const boardElem = document.getElementById("chess-board");
    const moveListElem = document.getElementById("replay-move-view");

    const moveListJson = document.getElementById("replay-move-list").innerHTML;
    const initialBoardJson = document.getElementById("initial-board").innerHTML;

    const moveList = JSON.parse(moveListJson);
    const initialBoard = JSON.parse(initialBoardJson);

    const boardCache = [];

    function moveInitialBoard(index) {
        if (boardCache[i] !== undefined) {
            return boardCache[i];
        }
        const board = structuredClone(initialBoard);
        for (let i = 0; i < index; i++) {
            const {to: {rank: toRank, file: toFile}, from: {rank: fromRank, file: fromFile}} = moveList[i];
            board[toRank][toFile] = board[fromRank][fromFile]
            board[fromRank][fromFile] = EMPTY;
        }
        boardCache[i] = board;
        return board;
    }

    function onClickMove(move) {

    }

    renderBoard(boardElem, initialBoard);
    renderMoveTable(moveListElem, moveList, onClickMove);
}
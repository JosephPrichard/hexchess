function renderReplay() {
    const boardElem = document.getElementById("chess-board");
    const moveListElem = document.getElementById("replay-move-list");

    const moveList = JSON.parse(document.getElementById("move-list-data").innerHTML);
    const initialBoard = JSON.parse(document.getElementById("initial-board-data").innerHTML);

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
    renderMoveList(moveListElem, moveList, onClickMove);
}
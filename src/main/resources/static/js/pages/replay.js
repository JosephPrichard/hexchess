function initReplay() {
    const moveList = JSON.parse(document.getElementById("replay-move-list").innerHTML);
    const initialBoard = JSON.parse(document.getElementById("initial-board").innerHTML);

    const boardCache = [];

    function moveInitialBoard(index) {
        if (boardCache[i] !== undefined) {
            return boardCache[i];
        }
        const board = structuredClone(initialBoard);
        for (let i = 0; i < index; i++) {
            const move = moveList[i];
            board[move.to.file][move.to.rank] = board[move.from.file][move.from.rank]
        }
        boardCache[i] = board;
        return board;
    }
}
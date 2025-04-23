async function renderAndAttachEventListeners(gameId) {
    const resp = await fetch("/static/initial-board", { method: "GET" });
    const initialBoard = await resp.json();

    const boardElement = document.getElementById("chess-board");

    renderBoard(boardElement, initialBoard, false);
}
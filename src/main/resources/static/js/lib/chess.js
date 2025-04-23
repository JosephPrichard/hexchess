function renderBoard(boardElement, board, isBlackPerspective) {
    boardElement.innerHTML = "";
    boardElement.style = `width: ${11 * HEX_HEIGHT}px; height: ${11 * HEX_HEIGHT}px`;

    for (let file = 0; file < board.pieces.length; file++) {
        const piecesFile = board.pieces[file];
        for (let rank = 0; rank < piecesFile.length; rank++) {
            const piece = piecesFile[rank];

            let top = ((rank * HEX_HEIGHT) + (VERTICAL_FILE_OFFSETS[file] * HEX_HEIGHT / 2));
            if (isBlackPerspective) {
                top = (10 * HEX_HEIGHT) - top;
            }

            const left = file * (HEX_HEIGHT - 8);

            const index = (COLOR_OFFSETS[file] + rank) % 3;
            const bgColor = COLORS[index];
            const piecename = PIECE_NAMES[piece] || EMPTY;

            boardElement.innerHTML += `
                <div
                    id="hexagon-${file}-${rank}"
                    data-rank="${rank}"
                    data-file="${file}"
                    class="hexagon"
                    style="
                        top: ${top}px;
                        left: ${left}px;
                        width: ${HEX_WIDTH}px;
                        height: ${HEX_HEIGHT}px;
                        background: ${bgColor};">
                    ${piecename !== EMPTY ? `
                        <img
                            id="piece-${file}-${rank}"
                            src="/static/images/pieces/${piecename}.png"
                            alt=""
                            draggable="false"
                            class="piece-img">` :
                        ""}
                </div>`;
        }
    }
}

function stringOfMove(move) {
    const symbol = PIECE_SYMBOLS[move.piece] || '?';
    const toFile = FILES[move.to.file];
    const toRank = move.to.rank + 1;

    return `${symbol}${toFile}${toRank}`;
}

function renderMoveList(moveListElem, moveList, handleOnClickMove) {
    let ply = 1;
    for (let i = 0; i < moveList.length; i += 2) {
        const firstMove = moveList[i];
        const secondMove = moveList[i + 1];

        const move = document.createElement("div");
        move.className = "move-row";

        const number = document.createElement("div");
        number.innerHTML = `${ply}.`;
        number.className = "move-number";

        const moveOne = document.createElement("div");
        moveOne.setAttribute("id", `move-${i}`)
        moveOne.innerHTML = stringOfMove(firstMove);
        moveOne.className = "move";
        moveOne.onclick = () => handleOnClickMove(i, moveOne);

        const moveTwo = document.createElement("div");
        moveTwo.setAttribute("id", `move-${i + 1}`)
        moveTwo.innerHTML = secondMove ? stringOfMove(secondMove) : "";
        moveTwo.className = "move";
        moveTwo.onclick = () => handleOnClickMove(i + 1, moveTwo);

        move.appendChild(number);
        move.appendChild(moveOne);
        move.appendChild(moveTwo);

        moveListElem.appendChild(move);

        ply++;
    }
}

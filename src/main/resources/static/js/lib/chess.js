const EMPTY = 0;
const WHITE_PAWN = 1;
const BLACK_PAWN = 2;
const WHITE_KNIGHT = 3;
const BLACK_KNIGHT = 4;
const WHITE_BISHOP = 5;
const BLACK_BISHOP = 6;
const WHITE_ROOK = 7;
const BLACK_ROOK = 8;
const WHITE_QUEEN = 9;
const BLACK_QUEEN = 10;
const WHITE_KING = 11;
const BLACK_KING = 12;

const pieceNames = {
    [WHITE_PAWN]: "white-pawn",
    [BLACK_PAWN]: "black-pawn",
    [WHITE_KNIGHT]: "white-knight",
    [BLACK_KNIGHT]: "black-knight",
    [WHITE_BISHOP]: "white-bishop",
    [BLACK_BISHOP]: "black-bishop",
    [WHITE_ROOK]: "white-rook",
    [BLACK_ROOK]: "black-rook",
    [WHITE_QUEEN]: "white-queen",
    [BLACK_QUEEN]: "black-queen",
    [WHITE_KING]: "white-king",
    [BLACK_KING]: "black-king"
};
const files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];
const pieceSymbols = {
    [WHITE_PAWN]: 'p',
    [BLACK_PAWN]: 'p',
    [WHITE_KNIGHT]: 'N',
    [BLACK_KNIGHT]: 'N',
    [WHITE_BISHOP]: 'B',
    [BLACK_BISHOP]: 'B',
    [WHITE_ROOK]: 'R',
    [BLACK_ROOK]: 'R',
    [WHITE_QUEEN]: 'Q',
    [BLACK_QUEEN]: 'Q',
    [WHITE_KING]: 'K',
    [BLACK_KING]: 'K',
};

const HEX_HEIGHT = 64;
const HEX_WIDTH = HEX_HEIGHT * 1.2;
const VERTICAL_FILE_OFFSETS = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5]; // file -> vertical offset

const LIGHT_COLOR = "rgb(255, 207, 159)";
const MEDIUM_COLOR = "rgb(233, 172, 112)";
const DARK_COLOR = "rgb(210,140,69)";

const COLORS = [LIGHT_COLOR, MEDIUM_COLOR, DARK_COLOR];
const COLOR_OFFSETS = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0]; // file -> color

function renderBoard(element, board, flip) {
    element.innerHTML = "";

    element.style.setProperty("width", `${11 * HEX_HEIGHT}px`);
    element.style.setProperty("height", `${11 * HEX_HEIGHT}px`);

    for (let file = 0; file < board.pieces.length; file++) {
        const piecesFile = board.pieces[file];
        for (let rank = 0; rank < piecesFile.length; rank++) {
            const piece = piecesFile[rank];

            let top = ((rank * HEX_HEIGHT) + (VERTICAL_FILE_OFFSETS[file] * HEX_HEIGHT / 2));
            if (flip) {
                top = (10 * HEX_HEIGHT) - top;
            }

            const left = file * (HEX_HEIGHT - 8);

            const index = (COLOR_OFFSETS[file] + rank) % 3;
            const bgColor = COLORS[index];

            const pieceElem = document.createElement("div");
            pieceElem.classList.add("hexagon");

            pieceElem.setAttribute("data-rank", String(rank));
            pieceElem.setAttribute("data-file", String(file));
            pieceElem.style.setProperty("top", `${top}px`);
            pieceElem.style.setProperty("left", `${left}px`);
            pieceElem.style.setProperty("width", `${HEX_WIDTH}px`);
            pieceElem.style.setProperty("height", `${HEX_HEIGHT}px`);
            pieceElem.style.setProperty("background", bgColor);

            if (piece !== EMPTY) {
                const piecename = pieceNames[piece] || "";

                const pieceImg = document.createElement("img");
                pieceImg.setAttribute("src", `/static/images/pieces/${piecename}.png`);
                pieceImg.setAttribute("alt", "");
                pieceImg.setAttribute("draggable", "false");
                pieceImg.classList.add("piece-img");

                pieceElem.appendChild(pieceImg);
            }

            element.appendChild(pieceElem);
        }
    }
}

function stringOfMove(move) {
    const symbol = pieceSymbols[move.piece] || '?';
    const toFile = files[move.to.file];
    const toRank = move.to.rank + 1;

    return `${symbol}${toFile}${toRank}`;
}
function renderMoveList(moveListElem, moveList, onClickMove) {
    let ply = 1;
    for (let i = 0; i < moveList.length; i += 2) {
        const moveOne = moveList[i];
        const moveTwo = moveList[i + 1];

        const moveElem = document.createElement("div");
        moveElem.classList.add("move-row");

        const numberElem = document.createElement("div");
        numberElem.appendChild(document.createTextNode(`${ply}.`));
        numberElem.classList.add("move-number");
        moveElem.appendChild(numberElem);

        const moveOneElem = document.createElement("div");
        moveOneElem.appendChild(document.createTextNode(stringOfMove(moveOne)));
        moveOneElem.classList.add("move-elem");
        moveElem.appendChild(moveOneElem);

        const moveTwoElem = document.createElement("div");
        moveTwoElem.appendChild(document.createTextNode(moveTwo ? stringOfMove(moveTwo) : ""));
        moveTwoElem.classList.add("move-elem");
        moveElem.appendChild(moveTwoElem);

        moveListElem.appendChild(moveElem);

        ply++;
    }
}


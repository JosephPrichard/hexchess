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

function fromPieceToName(piece) {
    switch (piece) {
        case WHITE_PAWN: return "white-pawn";
        case BLACK_PAWN: return "black-pawn";
        case WHITE_KNIGHT: return "white-knight";
        case BLACK_KNIGHT: return "black-knight";
        case WHITE_BISHOP: return "white-bishop";
        case BLACK_BISHOP: return "black-bishop";
        case WHITE_ROOK: return "white-rook";
        case BLACK_ROOK: return "black-rook";
        case WHITE_QUEEN: return "white-queen";
        case BLACK_QUEEN: return "black-queen";
        case WHITE_KING: return "white-king";
        case BLACK_KING: return "black-king";
        default: return "empty";
    }
}

const HEX_HEIGHT = 66;
const HEX_WIDTH = HEX_HEIGHT * 1.2;
const VERTICAL_FILE_OFFSETS = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5]; // file -> vertical offset

const LIGHT_COLOR = "rgb(255, 207, 159)";
const MEDIUM_COLOR = "rgb(233, 172, 112)";
const DARK_COLOR = "rgb(210,140,69)";

const COLORS = [LIGHT_COLOR, MEDIUM_COLOR, DARK_COLOR];
const COLOR_OFFSETS = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0]; // file -> color

function renderBoard(element, board) {
    element.innerHTML = "";

    element.style.setProperty("width", `${11 * HEX_HEIGHT}px`);
    element.style.setProperty("height", `${11 * HEX_HEIGHT}px`);

    for (let file = 0; file < board.length; file++) {
        const piecesFile = board[file];
        for (let rank = 0; rank < piecesFile.length; rank++) {
            const piece = piecesFile[rank];

            const top = (rank * HEX_HEIGHT) + (VERTICAL_FILE_OFFSETS[file] * HEX_HEIGHT / 2);
            const left = file * (HEX_HEIGHT - 8);

            const index = (COLOR_OFFSETS[file] + rank) % 3;
            const bgColor = COLORS[index];

            const pieceElem = document.createElement("div");
            pieceElem.classList.add("hexagon");

            pieceElem.style.setProperty("top", `${top}px`);
            pieceElem.style.setProperty("left", `${left}px`);
            pieceElem.style.setProperty("width", `${HEX_WIDTH}px`);
            pieceElem.style.setProperty("height", `${HEX_HEIGHT}px`);
            pieceElem.style.setProperty("background", bgColor);

            if (piece !== EMPTY) {
                const piecename = fromPieceToName(piece);

                const pieceImg = document.createElement("img");
                pieceImg.setAttribute("src", `/static/images/pieces/${piecename}.png`);
                pieceImg.setAttribute("alt", "");

                pieceElem.appendChild(pieceImg);
            }

            element.appendChild(pieceElem);
        }
    }
}

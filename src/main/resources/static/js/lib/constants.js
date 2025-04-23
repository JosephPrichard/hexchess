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

const PIECE_NAMES = {
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
const FILES = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];
const PIECE_SYMBOLS = {
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

const COLORS = ["rgb(255, 207, 159)", "rgb(233, 172, 112)", "rgb(210,140,69)"]; // LIGHT, MEDIUM, DARK
const COLOR_OFFSETS = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0]; // file -> color
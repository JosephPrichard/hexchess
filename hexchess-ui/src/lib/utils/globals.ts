import type { ChessBoard } from '$lib/api/messages';

export const env = 'PROD';

export const height = 64;
export const width = height * 1.2;
export const verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5];
export const colors = ['rgb(255, 207, 159)', 'rgb(233, 172, 112)', 'rgb(210,140,69)'];
export const selectedColor = 'rgb(140, 80, 49)';
export const colorsOffset = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0];
export const chessRowHeight = 45;
export const maxChessRows = 12;

export const piecenames: Record<number, string> = {
	0: 'empty',
	1: 'white-pawn',
	2: 'black-pawn',
	3: 'white-knight',
	4: 'black-knight',
	5: 'white-bishop',
	6: 'black-bishop',
	7: 'white-rook',
	8: 'black-rook',
	9: 'white-queen',
	10: 'black-queen',
	11: 'white-king',
	12: 'black-king'
};

export const initialBoard: ChessBoard = {
	isWhiteTurn: false,
	file: [
		{ pieces: [0,0,0,0,0,0] },
		{ pieces: [1,0,0,0,0,0,2] },
		{ pieces: [7,1,0,0,0,0,2,8] },
		{ pieces: [3,0,1,0,0,0,2,0,4] },
		{ pieces: [9,0,0,1,0,0,2,0,0,10] },
		{ pieces: [5,5,5,0,1,0,2,0,6,6,6] },
		{ pieces: [11,0,0,1,0,0,2,0,0,12] },
		{ pieces: [3,0,1,0,0,0,2,0,4] },
		{ pieces: [7,1,0,0,0,0,2,8] },
		{ pieces: [1,0,0,0,0,0,2] },
		{ pieces: [0,0,0,0,0,0] }
	]
}
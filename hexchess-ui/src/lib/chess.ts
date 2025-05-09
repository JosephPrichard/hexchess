import { type ChessBoard, type Move, Piece } from '$lib/models';

export function translateBoard(index: number, moveList: Move[], board: ChessBoard) {
    board = structuredClone(board)
    for (let i = 0; i <= index; i++) {
        const move = moveList![i];
        const {
            to: { rank: toRank, file: toFile },
            from: { rank: fromRank, file: fromFile }
        } = move;

        board.pieces[toFile][toRank] = board.pieces[fromFile][fromRank];
        board.pieces[fromFile][fromRank] = Piece.empty;
    }
    return board;
}
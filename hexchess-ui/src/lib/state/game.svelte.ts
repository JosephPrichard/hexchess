import type { Hex } from "$lib/api/models";
import type { ChessBoard, ChessGame } from "$lib/pb/messages";
import { makeGame, pieces, isInBounds, defaultGame } from "$lib/utils/chess";

export interface Promotion {
    from: Hex; 
    to: Hex;
};

export interface GameState {
    game?: ChessGame;
    prevGame?: ChessGame;
    promotion?: Promotion;
}

export function makeGameState() {
    let value: GameState = $state({});

    function updateGame(game: ChessGame) {
        value.promotion = undefined;
        value.prevGame = value.game;
        value.game = game;
    }

    function updateBoard(board: ChessBoard) {
        updateGame({
            ...(value.game || defaultGame),
            board
        });
    }
    
    function movePiece(from: Hex, to: Hex) {
        const board = $state.snapshot(value.game?.board);
        if (board &&
            (isInBounds(to) 
                && (from.file != to.file || from.rank != to.rank))
        ) {
            const piece = board.file[from.file].pieces[from.rank];
            board.file[from.file].pieces[from.rank] = pieces.empty;
            board.file[to.file].pieces[to.rank] = piece;

            updateBoard(board);
        }
    }
    
    function clear() {
        const board = $state.snapshot(value.game?.board);
        if (board) {
            for (const file of board.file) {
                file.pieces.fill(pieces.empty);
            }
            updateGame(makeGame(board));
        }
    }
    
    function removePiece(hex: Hex) {
        const board = $state.snapshot(value.game?.board);
        if (board) {
            board.file[hex.file].pieces[hex.rank] = pieces.empty;
            updateBoard(board);
        }
    }
    
    function placePiece(hex: Hex, piece: number) {
        const board = $state.snapshot(value.game?.board);
        if (isInBounds(hex) && board) {
            board.file[hex.file].pieces[hex.rank] = piece;
            updateBoard(board);
        }
    }
    
    function setTurn(turn: boolean) {
        const board = $state.snapshot(value.game?.board);
        if (board) {
            board.isWhiteTurn = turn;
            updateGame(makeGame(board));
        }
    }

    function setGame(game?: ChessGame) {
        updateGame(game || defaultGame);
    }

    function rollback() {
        if (value.prevGame !== undefined) {
            value.game = value.prevGame;
            value.prevGame = undefined;
        }
    }

    function setPromotion(promotion?: Promotion) {
		value.promotion = promotion;
	}
    
    return { value, movePiece, clear, removePiece, placePiece, setTurn, setGame, revert: rollback, setPromotion };
}
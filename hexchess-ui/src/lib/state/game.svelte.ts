import type { Hex } from '$lib/api/models';
import type { ChessBoard, ChessGame, HistMove } from '$lib/pb/messages';
import { defaultGame, isInBounds, makeGame, pieces } from '$lib/utils/chess';

export interface PromotionMove {
    from: Hex;
    to: Hex;
}

export interface GameState {
    game?: ChessGame;
    prevGame?: ChessGame;
    promotion?: PromotionMove;
}

export function makeGameState(game?: ChessGame) {
    let state: GameState = $state({ game });

    function setGame(game?: ChessGame) {
        if (!game) game = defaultGame;
        state.promotion = undefined;
        state.prevGame = state.game;
        state.game = game;
    }

    function setBoard(board: ChessBoard) {
        setGame({
            ...(state.game || defaultGame),
            board
        });
    }
    
    function movePiece(from: Hex, to: Hex) {
        const board = $state.snapshot(state.game?.board);
        if (board &&
            (isInBounds(to) 
                && (from.file != to.file || from.rank != to.rank))
        ) {
            const piece = board.file[from.file].pieces[from.rank];
            board.file[from.file].pieces[from.rank] = pieces.empty;
            board.file[to.file].pieces[to.rank] = piece;

            setBoard(board);
        }
    }
    
    function clear() {
        const board = $state.snapshot(state.game?.board);
        if (board) {
            for (const file of board.file) {
                file.pieces.fill(pieces.empty);
            }
            setGame(makeGame(board));
        }
    }
    
    function removePiece(hex: Hex) {
        const board = $state.snapshot(state.game?.board);
        if (board) {
            board.file[hex.file].pieces[hex.rank] = pieces.empty;
            setBoard(board);
        }
    }
    
    function placePiece(hex: Hex, piece: number) {
        const board = $state.snapshot(state.game?.board);
        if (isInBounds(hex) && board) {
            board.file[hex.file].pieces[hex.rank] = piece;
            setBoard(board);
        }
    }
    
    function setTurn(turn: boolean) {
        const board = $state.snapshot(state.game?.board);
        if (board) {
            board.isWhiteTurn = turn;
            setGame(makeGame(board));
        }
    }

    function revertPromotion() {
        if (state.prevGame !== undefined) {
            state.game = state.prevGame;
            state.prevGame = undefined;
        }
    }

    function setPromotion(promotion?: PromotionMove) {
        if (promotion) {
            movePiece(promotion.from, promotion.to);
        }
		state.promotion = promotion;
	}

    function getPrevMove() {
        const game = state.game;
        if (!game?.moves || game.moves.length == 0) {
            return undefined;
        }
        return game.moves[game.moves.length - 1];
    }
    
    return { state, getPrevMove, movePiece, clear, removePiece, placePiece, setTurn, setGame, revertPromotion, setPromotion };
}
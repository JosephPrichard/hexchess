import type { Hex } from '$lib/api/models';
import type { ChessGame } from '$lib/pb/messages';
import { type MakeMoveArgs, wasm } from '$lib/api/wasm';
import {chessService, defaultGame} from "$lib/service/chess";

export interface PromotionMove {
    from: Hex;
    to: Hex;
}

export interface PromotionState {
    move: PromotionMove;
    prevGame?: ChessGame;
}

export interface SandboxState {
    game?: ChessGame;
    promotion?: PromotionState;
}

export function makeSandboxState() {
    let state: SandboxState = $state({});

    function setGame(game: ChessGame) {
        if (!game) game = defaultGame;
        state.game = game;
    }

    function mutateGame(fn: (game: ChessGame) => ChessGame | undefined) {
        const game = fn($state.snapshot(state.game) || defaultGame);
        if (!game) return;
        setGame(game);
    }

    const movePiece = (from: Hex, to: Hex) =>
        mutateGame((g) => chessService.moveBoardPiece(g, from, to));

    const clear = () =>
        mutateGame((g) => chessService.clearBoard(g));

    const removePiece = (hex: Hex) =>
        mutateGame((g) => chessService.removeBoardPiece(g, hex));

    const placePiece = (hex: Hex, piece: number) =>
        mutateGame((g) => chessService.placeBoardPiece(g, hex, piece));

    const setTurn = (turn: boolean) =>
        mutateGame((g) => chessService.setBoardTurn(g, turn));

    function revertPromotion() {
        if (state.promotion !== undefined) {
            state.game = state.promotion.prevGame;
        }
        state.promotion = undefined;
    }

    function setPromotion(promotion?: PromotionMove) {
        if (promotion) {
            state.promotion = {move: promotion, prevGame: $state.snapshot(state.game)};
            movePiece(promotion.from, promotion.to);
        } else {
            state.promotion = undefined;
        }
    }

    async function makeMove(move: MakeMoveArgs) {
        revertPromotion();

        const nextGame = await wasm.makeMove($state.snapshot(state.game), move);
        if (nextGame === undefined) return false;

        setGame(nextGame);
        return true;
    }

    async function initNewBoard() {
        revertPromotion();

        const board = $state.snapshot(state.game?.board);
        if (!board) return;
        setGame(await wasm.getGame(board));
    }

    async function setInitialGame() {
        revertPromotion();
        setGame(await wasm.getGame());
    }

    return {
        state,
        movePiece,
        clear,
        removePiece,
        placePiece,
        setTurn,
        setGame,
        revertPromotion,
        setPromotion,
        makeMove,
        initNewBoard,
        setInitialGame,
    };
}
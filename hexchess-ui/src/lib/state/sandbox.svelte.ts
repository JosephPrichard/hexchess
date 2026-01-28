import type { Hex } from '$lib/api/models';
import type { ChessGame } from '$lib/pb/messages';
import { clearBoard, defaultGame, moveBoardPiece, placeBoardPiece, removeBoardPiece, setBoardTurn } from '$lib/service/chess';
import { type MakeMoveArgs, wasm } from '$lib/api/wasm';

export interface PromotionMove {
    from: Hex;
    to: Hex;
}

export interface SandboxState {
    game?: ChessGame;
    promotion?: {move: PromotionMove; game?: ChessGame};
}

export function makeSandboxState() {
    let state: SandboxState = $state({});

    function setGame(game: ChessGame) {
        if (!game) game = defaultGame;
        state.promotion = undefined;
        state.game = game;
    }

    function mutateGame(fn: (game: ChessGame) => ChessGame | undefined) {
        const game = fn($state.snapshot(state.game) || defaultGame);
        if (!game) return;
        setGame(game);
    }

    const movePiece = (from: Hex, to: Hex) => mutateGame((g) => moveBoardPiece(g, from, to));

    const clear = () => mutateGame((g) => clearBoard(g));

    const removePiece = (hex: Hex) => mutateGame((g) => removeBoardPiece(g, hex));

    const placePiece = (hex: Hex, piece: number) => mutateGame((g) => placeBoardPiece(g, hex, piece));

    const setTurn = (turn: boolean) => mutateGame((g) => setBoardTurn(g, turn));

    function revertPromotion() {
        if (state.promotion !== undefined) {
            state.game = state.promotion.game;
        }
        state.promotion = undefined;
    }

    function setPromotion(promotion?: PromotionMove) {
        if (promotion) {
            state.promotion = {move: promotion, game: $state.snapshot(state.game)};
            movePiece(promotion.from, promotion.to);
        }
        state.promotion = undefined;
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
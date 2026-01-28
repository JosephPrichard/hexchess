import { ChessBoard, ChessGame, HistMove, HistMoves, Move, PieceMove } from '$lib/pb/messages';
import type { Hex } from '$lib/api/models';
import { defaultBoard, defaultGame } from '$lib/service/chess';
import { browser } from '$app/environment';

declare const Go: any; // imported in the initWasm fn

let wasmReady: Promise<void>;

export function makeWasmAPI(): Promise<void> {
	if (!browser)
		return Promise.resolve();
	if (wasmReady)
		return wasmReady;
	wasmReady = new Promise(async (resolve) => {
		await import("/wasm/wasm_exec.js?url");
		const go = new Go();
		const wasm = await WebAssembly.instantiateStreaming(
			fetch("/static/wasm/chess.wasm"),
			go.importObject
		);
		go.run(wasm.instance);
		resolve();
	});
	return wasmReady;
}

function dynCall(name: string, ...args: unknown[]): unknown {
	const fn = (globalThis as Record<string, unknown>)[name];
	if (typeof fn !== "function")
		throw new Error(`dyn function ${name} not found`);
	return (fn as Function)(...args);
}

async function getGame(board?: ChessBoard): Promise<ChessGame> {
	await makeWasmAPI();

	const input = board ? ChessBoard.toBinary(board) : undefined;

	const output = dynCall("getGame", input);
	if (output instanceof Uint8Array) {
		return ChessGame.fromBinary(output);
	} else {
		console.error("failed to get initial game");
		return defaultGame;
	}
}

export type MakeMoveArgs = { from: Hex; to: Hex; promotion: number };

async function makeMove(
	game?: ChessGame,
	move?: MakeMoveArgs
): Promise<ChessGame | undefined> {
	await makeWasmAPI();

	const gameInput = ChessGame.toBinary(game || defaultGame);
	const moveInput = Move.toBinary({
		fromFile: move?.from?.file ?? 0,
		fromRank: move?.from?.rank ?? 0,
		toFile: move?.to?.file ?? 0,
		toRank: move?.to?.rank ?? 0,
		promotion: move?.promotion ?? 0
	});

	const output = dynCall("makeMove", gameInput, moveInput);
	if (output instanceof Uint8Array) {
		return ChessGame.fromBinary(output);
	}
	return undefined;
}

async function fenToGame(fen: string): Promise<ChessGame> {
	await makeWasmAPI();
	const output = dynCall("fenToGame", fen) as any;

	if (output[0] instanceof Uint8Array) {
		return ChessGame.fromBinary(output[0]);
	}

	let message = "failed to parse FEN string";
	if (typeof output[1] === "string") {
		message = output[1];
	}
	console.error(message);

	return defaultGame;
}

async function boardToFen(board?: ChessBoard): Promise<string> {
	if (!board) return "";

	await makeWasmAPI();
	const input = ChessBoard.toBinary(board);

	const output = dynCall("boardToFen", input);
	if (typeof output === "string") {
		return output;
	}

	console.error("failed to convert board to fen");
	return "";
}

async function gameAtMoveIndex(
	initialBoard: ChessBoard | undefined,
	moves: (HistMove | undefined)[] | undefined,
	index: number
): Promise<ChessGame> {
	await makeWasmAPI();

	initialBoard = initialBoard || defaultBoard;

	const movesList: HistMove[] = [];
	for (const item of moves ?? []) {
		if (item) movesList.push(item);
	}

	const inputMoves = HistMoves.toBinary({ moves: movesList });
	const output = dynCall("gameAtMoveIndex", ChessBoard.toBinary(initialBoard), inputMoves, index);

	if (output instanceof Uint8Array) {
		return ChessGame.fromBinary(output);
	}

	console.error("failed to make move");
	return defaultGame;
}

export const wasm = {
	getGame,
	makeMove,
	fenToGame,
	boardToFen,
	gameAtMoveIndex
};
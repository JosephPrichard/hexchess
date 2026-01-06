import { ChessBoard, ChessGame, HistMove, HistMoves, MakeMoveInput } from '$lib/pb/messages';
import type { Hex } from '$lib/api/models';
import { defaultGame } from '$lib/utils/chess.js';
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

export async function getInitialGameWasm(): Promise<ChessGame> {
	await makeWasmAPI();
	const output = dynCall("getInitialGame");
	if (output instanceof Uint8Array) {
		return ChessGame.fromBinary(output);
	}
	console.error("Failed to get initial game");
	return defaultGame;
}

export async function makeMoveWasm(game?: ChessGame, move?: { from: Hex, to: Hex, promotion: number }): Promise<ChessGame | undefined> {
	await makeWasmAPI();
	const input = MakeMoveInput.toBinary({
		game,
		move: move ? {
			fromFile: move.from.file,
			fromRank: move.from.rank,
			toFile: move.to.file,
			toRank: move.to.rank,
			promotion: move.promotion
		} : undefined
	});

	const output = dynCall("makeMove", input);
	if (output instanceof Uint8Array) {
		return ChessGame.fromBinary(output);
	}
	console.error("Failed to make move");
}

export async function getMovesWasm(board: ChessBoard): Promise<ChessGame | undefined> {
	await makeWasmAPI();
	const input = ChessBoard.toBinary(board);

	const output = dynCall("getMoves", input);
	if (output instanceof Uint8Array) {
		return ChessGame.fromBinary(output);
	}
	console.error("Failed to get moves");
}

export async function fenToGameWasm(fen: string): Promise<ChessGame | undefined> {
	await makeWasmAPI();
	const output = dynCall("fenToGame", fen) as any;
	if (output[0] instanceof Uint8Array) {
		return ChessGame.fromBinary(output[0]);
	}
	let message = "Failed to parse FEN string";
	if (typeof output[1] === "string") {
		message = output[1];
	}
	console.error(message);
}

export async function boardToFenWasm(board?: ChessBoard): Promise<string> {
	if (!board)
		return "";
	await makeWasmAPI();
	const input = ChessBoard.toBinary(board);

	const output = dynCall("boardToFen", input);
	if (typeof output === "string") {
		return output;
	}
	console.error("Failed to convert board to fen");
	return "";
}

export async function getMoveNotationsWasm(moves?: HistMove[]): Promise<string[]> {
	if (!moves || moves.length === 0)
		return [];
	await makeWasmAPI();
	const input = HistMoves.toBinary({ moves });

	const output = dynCall("getMoveNotations", input);
	if (Array.isArray(output) && output.every(item => typeof item === 'string')) {
		return output;
	}
	console.error("Failed to get move notations");
	return [];
}
import { ChessBoard, ChessGame, MakeMoveInput } from '$lib/api/messages';
import type { Hex } from '$lib/api/model';
import { defaultBoard, defaultGame, newGame, newMove } from '$lib/services/chess';

declare const Go: any; // imported in the initWasm fn

let wasmReady: Promise<void>;

export function initWasm(): Promise<void> {
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

export async function getInitialBoardDyn(): Promise<ChessBoard> {
	await initWasm();
	const out = dynCall("getInitialBoard") as (Uint8Array | undefined);
	if (out === undefined) {
		return defaultBoard;
	}
	const board = ChessBoard.fromBinary(out);
	console.log("getInitialBoard", board);
	return board;
}

export async function makeMoveDyn(board?: ChessBoard, move?: { from: Hex, to: Hex }): Promise<ChessGame> {
	await initWasm();
	const pm = move ? newMove(move.from, move.from) : undefined;
	const bin = MakeMoveInput.toBinary({ board, move: pm });
	const out = dynCall("makeMove", bin) as (Uint8Array | undefined);
	if (out === undefined) {
		return newGame(defaultBoard);
	}
	const game = ChessGame.fromBinary(out);
	console.log("makeMove", game);
	return game;
}

interface FenToGameResult {
	err?: string;
	game?: Uint8Array;
}

export async function fenToGameDyn(fen: string): Promise<{game?: ChessGame, err?: string}> {
	await initWasm();
	const result = dynCall("fenToGame", fen) as FenToGameResult | undefined;
	console.log("fenToGame", result);
	if (result === undefined) {
		return { err: "Failed to parse Fen to board" };
	}
	const game = result.game ? ChessGame.fromBinary(result.game) : defaultGame;
	return { game };
}

export async function boardToFenDyn(board?: ChessBoard): Promise<string> {
	if (!board)
		return "";
	await initWasm();
	const bin = ChessBoard.toBinary(board);
	const fen = (dynCall("boardToFen", bin) || "") as string;
	console.log("boardToFenDyn", fen);
	return fen;
}

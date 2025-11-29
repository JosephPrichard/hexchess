import { ChessBoard, ChessGame, HistMove, HistMoves, MakeMoveInput } from '$lib/pb/messages';
import type { Hex } from '$lib/api/models';
import {defaultGame } from '$lib/utils/chess.js';
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
	const out = dynCall("getInitialGame") as (Uint8Array | undefined);
	if (out === undefined) {
		console.error("Failed to get initial game");
		return defaultGame;
	}
	return ChessGame.fromBinary(out);
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

	const output = dynCall("makeMove", input) as (Uint8Array | undefined);
	if (output === undefined) {
		return undefined;
	}
	return ChessGame.fromBinary(output);
}

export async function getMovesWasm(board: ChessBoard): Promise<ChessGame | undefined> {
	await makeWasmAPI();
	const input = ChessBoard.toBinary(board);

	const output = dynCall("getMoves", input) as (Uint8Array | undefined);
	if (output === undefined) {
		return undefined;
	}
	return ChessGame.fromBinary(output);
}

interface FenToGameResult {
	err?: string;
	game?: Uint8Array;
}

export async function fenToGameWasm(fen: string): Promise<ChessGame | string> {
	await makeWasmAPI();
	const output = dynCall("fenToGame", fen) as FenToGameResult | undefined;
	if (output === undefined) {
		return "Failed to parse fen to board";
	}
	return output.game ? ChessGame.fromBinary(output.game) : defaultGame;
}

export async function boardToFenWasm(board?: ChessBoard): Promise<string> {
	if (!board)
		return "";
	await makeWasmAPI();
	const input = ChessBoard.toBinary(board);
	return (dynCall("boardToFen", input) || "") as string;
}

export async function getMoveNotationsWasm(moves?: HistMove[]): Promise<string[]> {
	if (!moves || moves.length === 0)
		return [];
	await makeWasmAPI();
	const input = HistMoves.toBinary({ moves });
	return (dynCall("getMoveNotations", input) || []) as string[];
}
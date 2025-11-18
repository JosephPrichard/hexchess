import { ChessBoard, ChessGame, MakeMoveInput } from '$lib/api/messages';
import type { Hex } from '$lib/api/model';

declare const Go: any; // imported in the initWasm fn

let wasmReady: Promise<void>;

export function initWasm(): Promise<void> {
	if (wasmReady) return wasmReady;
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

export interface FenToBoardResult {
	err?: string;
	board?: Uint8Array;
}

export async function getInitialBoardDyn(): Promise<ChessBoard> {
	await initWasm();
	const out = dynCall("getInitialBoard") as Uint8Array;
	return ChessBoard.fromBinary(out);
}

export async function makeMoveDyn(board?: ChessBoard, move?: { from: Hex, to: Hex }): Promise<ChessGame> {
	await initWasm();
	const pm = move
		? { piece: 0, fromFile: move.from.file, fromRank: move.from.rank, toFile: move.to.file, toRank: move.to.rank }
		: undefined;
	const bin = MakeMoveInput.toBinary({ board, move: pm });
	const out = dynCall("makeMove", bin) as Uint8Array;
	return ChessGame.fromBinary(out);
}

export async function fenToBoardDyn(fen: string): Promise<FenToBoardResult> {
	await initWasm();
	return dynCall("fenToBoard", fen) as FenToBoardResult;
}

export async function boardToFenDyn(board?: ChessBoard): Promise<string> {
	if (!board)
		return "";
	await initWasm();
	const bin = ChessBoard.toBinary(board);
	return dynCall("boardToFen", bin) as string;
}

import type { ChessGame, HistMove } from '$lib/pb/messages';
import { GameModeTimers } from '$lib/api/models';
import { assertHistMove } from '$lib/utils/asserts';
import { wasm } from '$lib/api/wasm';

export interface TimerType {
	endWhiteTimerMs: number;
	endBlackTimerMs: number;
	whiteTimerMs: number;
	blackTimerMs: number;
	diffMs: number;
}

export type CancelCountdown = () => void;

export function startCountdown(durationMs: number, onTick: (remaining: number) => void, onComplete: () => void): CancelCountdown {
	let start = performance.now();
	let remaining = durationMs;
	let timerId: number;

	function tick(now: number) {
		let passed = now - start;
		remaining = Math.max(0, durationMs - passed);

		onTick(remaining);

		if (remaining > 0) {
			timerId = requestAnimationFrame(tick);
		} else {
			onComplete();
		}
	}

	timerId = requestAnimationFrame(tick);

	return () => cancelAnimationFrame(timerId);
}

// export function startCountdown(durationMs: number, onTick: (passed: number) => void, onComplete: () => void): CancelCountdown {
// 	const timerId = window.setTimeout(onComplete, durationMs);
// 	return () => clearTimeout(timerId);
// }

export async function gameAtStepIndex(initialGame: ChessGame | undefined, steps: HistMove[], stepIndex: number | undefined) {
	if (stepIndex !== undefined) {
		return await wasm.gameAtMoveIndex(initialGame?.board, steps, stepIndex);
	} else {
		return initialGame
	}
}

export function getStepTimers(stepIndex: number | undefined, steps: HistMove[], mode: string): TimerType | undefined {
	if (steps.length == 0) return;

	const startTimer = GameModeTimers.get(mode);
	if (!startTimer) return;

	if (stepIndex !== undefined) {
		const currHm = assertHistMove(steps[stepIndex]);
		const nextHm = steps[stepIndex + 1];

		let diffMs = 0; // no next move means we're at the last move, so there is no diff.
		if (nextHm) {
			const isWhiteTurn = stepIndex % 2 == 0;
			diffMs = Number(!isWhiteTurn ? currHm.whiteTimerMs - nextHm.whiteTimerMs : currHm.blackTimerMs - nextHm.blackTimerMs)
		}

		const whiteTimerMs = Number(currHm.whiteTimerMs);
		const blackTimerMs = Number(currHm.blackTimerMs);

		return { whiteTimerMs, blackTimerMs, endWhiteTimerMs: whiteTimerMs-diffMs, endBlackTimerMs: blackTimerMs-diffMs, diffMs };
	} else {
		const nextHm = assertHistMove(steps[0]);
		const diffMs =  startTimer - Number(nextHm.whiteTimerMs); // at the initial state, the first move is always white.

		return { whiteTimerMs: startTimer, blackTimerMs: startTimer, endWhiteTimerMs: startTimer-diffMs, endBlackTimerMs: startTimer-diffMs, diffMs};
	}
}
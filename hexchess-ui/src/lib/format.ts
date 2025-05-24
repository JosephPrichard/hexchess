import type { ReplayCause, ReplayResult, TimeControl } from '$lib/models';

export function getResultClasses(result: ReplayResult) {
	switch (result) {
		case 'WHITE_WIN':
			return ['green-color', 'red-color'];
		case 'BLACK_WIN':
			return ['red-color', 'green-color'];
		case 'DRAW':
			return ['yellow-color', 'yellow-color'];
		default:
			console.error('Unknown result case', result);
			return ['', ''];
	}
}

export function getWinrateClass(winRate: number) {
	if (winRate > 50) {
		return 'green-color';
	} else if (winRate < 50) {
		return 'red-color';
	} else {
		return 'yellow-color';
	}
}

export function formatReplayResult(result: ReplayResult) {
	switch (result) {
		case 'WHITE_WIN':
			return 'White Victory';
		case 'BLACK_WIN':
			return 'Black Victory';
		case 'DRAW':
			return 'Draw';
		default:
			console.error('Unknown result case', result);
			return '';
	}
}

export function formatCause(cause: ReplayCause) {
	switch (cause) {
		case 'CHECKMATE':
			return 'Checkmate';
		case 'FORFEIT':
			return 'Forfeit';
		default:
			console.error('Unknown result cause', cause);
			return '';
	}
}

export function formatTimeControl(timeControl: TimeControl) {
	switch (timeControl) {
		case 'UNLIMITED':
			return 'Unlimited ∞+0';
		case 'CORRESPONDENCE':
			return `Correspondence ${10}+${1}`;
		case 'REAL_TIME':
			return `Realtime ${5}+${3}`;
		default:
			throw new Error('Invalid time control: ' + timeControl);
	}
}

export function formatElo(elo: number) {
	return (elo >= 0 ? '+' : '') + elo;
}

export function formatTimer(ms: number) {
	const minutes = String(Math.floor(ms / 60000)).padStart(2, '0');
	ms %= 60000;
	const seconds = String(Math.floor(ms / 1000)).padStart(2, '0');
	ms %= 1000;
	return `${minutes}:${seconds}:${String(ms).padStart(2, '0')}`;
}

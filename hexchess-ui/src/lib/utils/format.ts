import type { TimeControl } from '$lib/api/model';

export function getResultClasses(result: number) {
	switch (result) {
		case 0:
			return ['green-color', 'red-color'];
		case 1:
			return ['red-color', 'green-color'];
		case 2:
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

export function formatReplayResult(result: number) {
	switch (result) {
		case 0:
			return 'White Victory';
		case 1:
			return 'Black Victory';
		case 2:
			return 'Draw';
		default:
			console.error('Unknown result case', result);
			return '';
	}
}

export function formatCause(cause: number) {
	switch (cause) {
		case 0:
			return 'Checkmate';
		case 1:
			return 'Forfeit';
		default:
			console.error('Unknown result cause', cause);
			return '';
	}
}

export const timeControlIntMap: TimeControl[] = ['UNLIMITED', 'CORRESPONDENCE', 'REAL_TIME'];

export function formatTimeControl(timeControl: number) {
	switch (timeControl) {
		case 0:
			return 'Unlimited ∞+0';
		case 1:
			return `Correspondence ${10}+${1}`;
		case 2:
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

export function formatTimestamp(ts: string): string {
	const timestamp = new Date(ts);
	const mm = String(timestamp.getMonth() + 1).padStart(2, "0");
	const dd = String(timestamp.getDate()).padStart(2, "0");
	const yyyy = timestamp.getFullYear();
	return `${mm}/${dd}/${yyyy}`;
}

export function formatJoinedOn(ts: string): string {
	return formatTimestamp(ts)
}

export function formatPlayedOn(ts: string): string {
	const timestamp = new Date(ts);

	const now = Date.now();
	const then = timestamp.getTime();
	const diffMs = now - then;

	const diffMinutes = Math.floor(diffMs / (1000 * 60));
	const diffHours = Math.floor(diffMinutes / 60);
	const diffDays = Math.floor(diffHours / 24);

	if (diffDays > 0) {
		return timestamp.toLocaleString();
	} else if (diffHours > 1) {
		return `${diffHours} hours ago`;
	} else if (diffHours === 1) {
		return "1 hour ago";
	} else if (diffMinutes === 1) {
		return "1 minute ago";
	} else {
		return `${diffMinutes} minutes ago`;
	}
}

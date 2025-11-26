export function getResultClasses(result: string) {
	switch (result) {
		case 'WHITE_WINS':
			return ['green-color', 'red-color'];
		case 'BLACK_WINS':
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

export function formatReplayResult(result: string) {
	switch (result) {
		case 'WHITE_WINS':
			return 'White Victory';
		case 'BLACK_WINS':
			return 'Black Victory';
		case 'DRAW':
			return 'Draw';
		default:
			console.error('Unknown replay result', result);
			return '';
	}
}

export function formatTimeControl(timeControl: string) {
	switch (timeControl) {
		case "UNLIMITED":
			return 'Unlimited ∞+0';
		case "CORRESPONDENCE":
			return `Correspondence ${10}+${1}`;
		case "REAL_TIME":
			return `Realtime ${5}+${3}`;
		default:
			console.error('Unknown time control: ' + timeControl);
			return '-';
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

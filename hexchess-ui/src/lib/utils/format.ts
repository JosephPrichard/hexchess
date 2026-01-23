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

export function formatResult(result: string) {
	switch (result) {
		case 'WHITE_WINS':
			return "White Won";
		case 'BLACK_WINS':
			return "Black Won";
		case 'DRAW':
			return "Draw";
		default:
			return "-";
	}
}

export function formatCause(cause: string) {
	switch (cause) {
		case 'CHECKMATE':
			return "checkmate";
		case 'FORFEIT':
			return "forfeit";
		case 'STALEMATE':
			return "stalemate";
		default:
			return "-";
	}
}


export function getReplayColors(result: string) {
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

export function formatEloDiff(elo: number) {
	return (elo >= 0 ? '+' : '') + Math.round(elo);
}

export function formatTimer(ms: number) {
	const minutes = String(Math.floor(ms / 60000)).padStart(2, '0');
	ms %= 60000;
	const seconds = String(Math.floor(ms / 1000)).padStart(2, '0');
	ms %= 1000;
	return `${minutes}:${seconds}:${String(ms).padStart(2, '0')}`;
}

export function formatTimestamp(ts: Date | string | number): string {
	const timestamp = new Date(ts);
	const mm = String(timestamp.getMonth() + 1).padStart(2, "0");
	const dd = String(timestamp.getDate()).padStart(2, "0");
	const yyyy = timestamp.getFullYear();
	return `${mm}/${dd}/${yyyy}`;
}

export function formatJoinedOn(ts: Date | string | number): string {
	return formatTimestamp(ts)
}

export function formatPlayedOn(ts: Date | string | number): string {
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

export function formatRelativeTime(ts: string, now: Date = new Date()): string {
	const target = new Date(ts)
	const diffMs = target.getTime() - now.getTime()

	const seconds = Math.round(diffMs / 1000)
	const minutes = Math.round(seconds / 60)
	const hours = Math.round(minutes / 60)
	const days = Math.round(hours / 24)

	const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" })

	if (Math.abs(seconds) < 60) {
		return rtf.format(seconds, "second")
	}
	if (Math.abs(minutes) < 60) {
		return rtf.format(minutes, "minute")
	}
	if (Math.abs(hours) < 24) {
		return rtf.format(hours, "hour")
	}

	return rtf.format(days, "day")
}

export function normalizeToDay(input: Date | string | number): string {
	const d = new Date(input);
	d.setHours(0, 0, 0, 0);
	return d.toISOString();
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
import type { HistMove } from '$lib/pb/messages';

export function assertHistMove(hm?: HistMove) {
	if (!hm) throw new Error('move history should not defined');
	return hm;
}
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getInitialBoard, getReplay, unwrap } from '$lib/api';
import { createMessage } from '$lib/error';
import type { ReplayProps } from './+page.svelte';

export const load: PageServerLoad = async ({ params, setHeaders, fetch }): Promise<ReplayProps> => {
	const id = params.id;

	const [initialBoard, replayResult] = await Promise.all([getInitialBoard(fetch), unwrap(getReplay(id, fetch))]);

	if (!initialBoard) {
		error(500, 'Unexpected error has occurred.');
	}

	if (!replayResult.ok || replayResult.resp === undefined) {
		error(replayResult.status, createMessage(replayResult.err));
	}

	setHeaders({
		'cache-control': 'max-age=3600'
	});
	return { replay: replayResult.resp, initialBoard };
};

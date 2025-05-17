import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getUserWithReplays, unwrap } from '$lib/api';
import { createMessage } from '$lib/error';
import type { PlayerProps } from './+page.svelte';

export const load: PageServerLoad = async ({ params, setHeaders, fetch }): Promise<PlayerProps> => {
	const id = params.id;

	const { ok, status, err, resp } = await unwrap(getUserWithReplays(id, fetch));

	if (!ok || resp === undefined) {
		error(status, createMessage(err));
	}

	setHeaders({
		'cache-control': 'max-age=300'
	});
	return { userWithReplays: resp };
};

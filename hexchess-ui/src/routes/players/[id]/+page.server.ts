import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import services from '$lib/api/services';
import { createMessage } from '$lib/utils/error';
import type { PlayerProps } from './+page.svelte';

export const load: PageServerLoad = async ({ params, setHeaders, fetch }): Promise<PlayerProps> => {
	const id = params.id;

	const [data, err] = await services.getUserWithReplays(id, fetch);

	if (err || data === undefined) {
		error(err?.status || 500, createMessage(err));
	}

	// setHeaders({
	// 	'cache-control': 'max-age=300'
	// });
	return { userWithReplays: data };
};

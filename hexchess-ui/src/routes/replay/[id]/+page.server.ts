import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { createMessage } from '$lib/services/error';
import type { ReplayProps } from './+page.svelte';
import services from '$lib/api/services';

export const load: PageServerLoad = async ({ params, setHeaders, fetch }): Promise<ReplayProps> => {
	const id = params.id;

	const [data, err] = await services.getReplay(id, fetch);

	if (err || data === undefined) {
		error(err?.status || 500, createMessage(err));
	}

	// setHeaders({
	// 	'cache-control': 'max-age=3600'
	// });
	return { replay: data.replay };
};

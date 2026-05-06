import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import {codes, makeMessage} from '$lib/utils/error';
import type { ReplayProps } from './+page.svelte';
import services from '$lib/api/services';

export const load: PageServerLoad = async ({ params, setHeaders, fetch }): Promise<ReplayProps> => {
	const id = params.id;

	const [data, err] = await services.getReplay(id, undefined, fetch);

	if (err?.message === codes.errorNotFoundReplay) {
		error(err?.status || 404, makeMessage(err));
	}
	if (err || data === undefined) {
		error(err?.status || 500, makeMessage(err));
	}

	// setHeaders({
	// 	'cache-control': 'max-age=3600'
	// });
	return { replay: data.replay };
};

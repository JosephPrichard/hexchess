import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import services from '$lib/api/services';
import { handleError } from '$lib/utils/error';
import type { PlayerProps } from './+page.svelte';

export const load: PageServerLoad = async ({ params, setHeaders, fetch }): Promise<PlayerProps> => {
	const id = params.id;

	const [data, err] = await services.getUser(id, true, fetch);

	if (err?.message === 'USER_NOT_FOUND') {
		error(err?.status || 404, handleError(err));
	}
	if (err || data === undefined) {
		error(err?.status || 500, handleError(err));
	}

	// setHeaders({
	// 	'cache-control': 'max-age=300'
	// });
	return { fullUser: data };
};

import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import services from '$lib/api/services';
import { makeMessage } from '$lib/utils/error';
import type { LeaderboardProps } from './+page.svelte';

export const load: PageServerLoad = async ({ url, setHeaders, fetch }): Promise<LeaderboardProps> => {
	const page = Number(url.searchParams.get('page') || 1);
	if (isNaN(page)) {
		error(404, 'Page must be a valid number');
	}

	const [data, err] = await services.getLeaderboard(page, fetch);
	if (err) {
		error(err.status, makeMessage(err));
	}

	// setHeaders({
	// 	'cache-control': 'max-age=60'
	// });
	return { page, pageCount: data?.totalPages || 0, userList: data?.userList || [] };
};

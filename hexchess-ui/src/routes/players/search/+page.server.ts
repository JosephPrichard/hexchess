import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import services from '$lib/api/services';
import { codes, makeMessage } from '$lib/utils/error';
import type { SearchProps } from './+page.svelte';

export const load: PageServerLoad = async ({ url, setHeaders, fetch }): Promise<SearchProps> => {
	const username = url.searchParams.get('username') || '';
	const page = Number(url.searchParams.get('page') || 1);
	if (username === null) {
		return { searchText: '', page: 1, userList: [], message: "" };
	}
	if (isNaN(page)) {
		error(404, 'Page must be a valid number');
	}

	let message = "";

	const [data, err] = await services.getSearchPlayers(username, page, fetch);
	if (err && err.message === codes.errorSearchLimit) {
		message = makeMessage(err);
	} else if (err) {
		error(err.status, makeMessage(err));
	}

	// setHeaders({
	// 	'cache-control': 'max-age=3600'
	// });
	return { searchText: username, page: page, userList: data?.userList || [], message: message };
};

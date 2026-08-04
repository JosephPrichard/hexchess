import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { services } from '$lib/api/services';
import { codes, handleError } from '$lib/utils/error';
import { env as publicEnv } from '$env/dynamic/public';
import type { LeaderboardUser } from '$lib/api/models';

export interface SearchProps {
	searchText: string;
	page: number;
	userList: LeaderboardUser[];
	message: string;
}

export const load: PageServerLoad = async (event): Promise<SearchProps> => {
	const username = event.url.searchParams.get('username') || '';
	const page = Number(event.url.searchParams.get('page') || 1);
	if (username === null) {
		return { searchText: '', page: 1, userList: [], message: "" };
	}
	if (isNaN(page)) {
		error(404, 'Page must be a valid number');
	}

	let message = "";

	const [data, err] = await services.getSearchPlayers(username, page);
	if (err && err.message === codes.errorSearchLimit) {
		message = handleError(err);
	} else if (err) {
		error(err.status, handleError(err));
	}


	if (publicEnv.PUBLIC_ACTIVE_PROFILE == "prod") {
		event.setHeaders({ 'cache-control': 'max-age=3600' });
	}
	return { searchText: username, page: page, userList: data?.userList ?? [], message: message };
};

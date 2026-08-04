import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { services } from '$lib/api/services';
import { handleError } from '$lib/utils/error';
import { env as publicEnv } from '$env/dynamic/public';
import type { LeaderboardUser } from '$lib/api/models';

export interface LeaderboardProps {
	page: number;
	pageCount: number;
	userList: LeaderboardUser[];
}

export const load: PageServerLoad = async (event): Promise<LeaderboardProps> => {
	const mode = event.url.searchParams.get('mode') || 'CORRESPONDENCE_7';
	const page = Number(event.url.searchParams.get('page') || 1);
	if (isNaN(page)) {
		error(404, 'Page must be a valid number');
	}

	const [data, err] = await services.getLeaderboard(page, mode);
	if (err) {
		error(err.status, handleError(err));
	}

	if (publicEnv.PUBLIC_ACTIVE_PROFILE == "prod") {
		event.setHeaders({ 'cache-control': 'max-age=60' });
	}
	return { 
		page, 
		pageCount: data?.totalPages ?? 0, 
		userList: data?.userList ?? []
	};
};

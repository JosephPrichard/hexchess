import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getLeaderboard } from '$lib/api';
import { createMessage } from '$lib/response';
import type { LeaderboardProps } from './+page.svelte';

export const load: PageServerLoad = async ({ url }): Promise<LeaderboardProps> => {
    const page = Number(url.searchParams.get('page'));

    if (isNaN(page)) {
        error(404, 'Page must be a valid number');
    }

    const { ok, status, err, resp } = await getLeaderboard(page);

    if (!ok) {
        error(status, createMessage(err));
    }

    return { page, pageCount: resp?.totalPages || 0, userList: resp?.userList || [] };
};
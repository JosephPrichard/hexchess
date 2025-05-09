import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getLeaderboard, unwrap } from '$lib/api';
import { createMessage } from '$lib/response';
import type { LeaderboardProps } from './+page.svelte';

export const load: PageServerLoad = async ({ url, setHeaders }): Promise<LeaderboardProps> => {
    const page = Number(url.searchParams.get('page'));
    if (isNaN(page)) {
        error(404, 'Page must be a valid number');
    }

    const { ok, status, err, resp } = await unwrap(getLeaderboard(page));
    if (!ok || resp === undefined) {
        error(status, createMessage(err));
    }

    setHeaders({
        'cache-control': 'max-age=60'
    });
    return { page, pageCount: resp?.totalPages || 0, userList: resp?.userList || [] };
};
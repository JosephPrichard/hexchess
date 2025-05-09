import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getSearchPlayers, unwrap } from '$lib/api';
import { createMessage } from '$lib/response';
import type { SearchProps } from './+page.svelte';

export const load: PageServerLoad = async ({ url, setHeaders }): Promise<SearchProps> => {
    const username = url.searchParams.get('username');
    const page = Number(url.searchParams.get('page'));
    if (username === null) {
        return { searchText: "", page: 1, userList: [] };
    }
    if (isNaN(page)) {
        error(404, 'Page must be a valid number');
    }

    const { ok, status, err, resp } = await unwrap(getSearchPlayers(username, page));
    if (!ok || resp === undefined) {
        error(status, createMessage(err));
    }

    setHeaders({
        'cache-control': 'max-age=3600'
    });
    return { searchText: username, page: page, userList: resp || [] };
};
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getSearchPlayers } from '$lib/api';
import { createMessage } from '$lib/response';
import type { SearchProps } from './+page.svelte';

export const load: PageServerLoad = async ({ url }): Promise<SearchProps> => {
    const username = url.searchParams.get('username');
    const page = Number(url.searchParams.get('page'));

    if (username === null) {
        return { searchText: "", page: 1, userList: [] };
    }

    if (isNaN(page)) {
        error(404, 'Page must be a valid number');
    }

    const { ok, status, err, resp } = await getSearchPlayers(username, page);

    if (!ok || resp === undefined) {
        error(status, createMessage(err));
    }

    return { searchText: username, page: page, userList: resp || [] };
};
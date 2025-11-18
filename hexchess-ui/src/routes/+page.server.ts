import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { createMessage } from '$lib/services/error';
import services from '$lib/api/services';
import type { IndexProps } from './+page.svelte';
import { maxChessRows } from '$lib/services/render';

export const load: PageServerLoad = async ({ url, fetch }): Promise<IndexProps> => {
	const page = Number(url.searchParams.get('page') || 1);
	if (isNaN(page)) {
		error(404, 'Page must be a valid number');
	}

	const [data, err] = await services.getChessRooms(maxChessRows, page, fetch);

	if (err || data === undefined) {
		error(err?.status || 500, createMessage(err));
	}

	return { chessList: data?.chessList, selfChessList: data?.selfChessList };
};
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { handleError } from '$lib/utils/error';
import services from '$lib/api/services';
import type { IndexProps } from './+page.svelte';
import { maxChessRows } from './globals';

export const load: PageServerLoad = async ({ url, fetch }): Promise<IndexProps> => {
	const fen = url.searchParams.get('fen') || "";
	const showCreateModal = url.searchParams.get('showCreateModal') === 'true';
	const page = Number(url.searchParams.get('page') || 1);
	if (isNaN(page)) {
		error(404, 'Page must be a valid number');
	}

	const [data, err] = await services.getGameRooms(maxChessRows, page, fetch);
	if (err || data === undefined) {
		error(err?.status || 500, handleError(err));
	}

	return {
		chessList: data?.chessList,
		selfChessList: data?.selfChessList,
		showCreateModal,
		fen
	};
};
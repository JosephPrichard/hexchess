import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { handleError } from '$lib/utils/error';
import { services } from '$lib/api/services';
import { maxChessRows } from './globals';
import type { ChessMetadata } from '$lib/api/models';

export interface IndexProps {
	chessList: ChessMetadata[];
	selfChessList: ChessMetadata[];
	showCreateModal?: boolean;
	fen?: string;
}

export const load: PageServerLoad = async (event): Promise<IndexProps> => {
	const fen = event.url.searchParams.get('fen') || "";
	const showCreateModal = event.url.searchParams.get('showCreateModal') === 'true';
	const page = Number(event.url.searchParams.get('page') || 1);
	if (isNaN(page)) {
		error(404, 'Page must be a valid number');
	}

	const [data, err] = await services.getGameRooms(maxChessRows, page);
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
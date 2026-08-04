import type { PageServerLoad } from './$types';
import { services } from '$lib/api/services';

export interface PlayProps {
	gameId: string
	gameExists?: boolean
}

export const load: PageServerLoad = async (event): Promise<PlayProps> => {
	const gameId = event.params.id || '';

	const [data, err] = await services.getGameExistence(gameId);
	const exists = data?.message != "GAME_NOT_EXISTS" && !err;

	return { gameId, gameExists: exists };
};
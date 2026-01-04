import type { PageLoad } from './$types';
import type { PlayProps } from './+page.svelte';
import services from '$lib/api/services';

export const load: PageLoad = async ({ params }): Promise<PlayProps> => {
	const gameId = params.id || '';

	const [data, err] = await services.getGameExistence(gameId);
	const exists = data?.message == "GAME_NOT_EXISTS" && !err;

	return { gameId, gameExists: exists };
};
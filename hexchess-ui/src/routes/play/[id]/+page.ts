import type { PageLoad } from './$types';
import type { PlayProps } from './+page.svelte';

export const load: PageLoad = async ({ params }): Promise<PlayProps> => {
	const id = params.id || '';
	return { gameId: id };
};

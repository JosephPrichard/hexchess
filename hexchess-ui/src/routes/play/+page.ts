import type { PageLoad } from './$types';
import type { PlayProps } from './+page.svelte';

export const load: PageLoad = async ({ url }): Promise<PlayProps> => {
	const id = url.searchParams.get('id') || '';
	return { gameId: id };
};

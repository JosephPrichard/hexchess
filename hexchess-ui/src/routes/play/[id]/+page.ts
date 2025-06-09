import type { PageLoad } from './$types';
import type { PlayProps } from './+page.svelte';

export const load: PageLoad = async ({ params }): Promise<PlayProps> => ({ gameId: params.id || '' });
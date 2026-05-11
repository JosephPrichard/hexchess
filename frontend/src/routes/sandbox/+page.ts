import type { PageLoad } from './$types';
import type { SandboxProps } from './+page.svelte';

export const load: PageLoad = async ({ url }): Promise<SandboxProps> => {
	const fen = url.searchParams.get('fen') || '';
	return { fen };
};

import type { PageLoad } from './$types';

export interface SandboxProps {
	fen: string;
}

export const load: PageLoad = async ({ url }): Promise<SandboxProps> => {
	const fen = url.searchParams.get('fen') || '';
	return { fen };
};

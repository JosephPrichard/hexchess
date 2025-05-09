import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';
import type { ChallengeProps } from './+page.svelte';

export const load: PageLoad = async ({ url }): Promise<ChallengeProps> => {
    const participants = url.searchParams.get('participants');
    if (participants == null) {
        error(404, 'Participants query parameter is required');
    }

    return { participants };
};
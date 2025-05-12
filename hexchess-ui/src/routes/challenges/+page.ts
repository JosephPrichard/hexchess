import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';
import type { ChallengeProps } from './+page.svelte';
import { getChallenges, unwrap } from '$lib/api';
import { createMessage } from '$lib/error';

export const load: PageLoad = async ({ url, fetch }): Promise<ChallengeProps> => {
    const participants = url.searchParams.get('participants') || 'sent';

    const { ok, status, err, resp } = await unwrap(getChallenges(participants, fetch));
    if (!ok || resp === undefined) {
        error(status, createMessage(err));
    }

    return { participants, challengeList: resp };
};
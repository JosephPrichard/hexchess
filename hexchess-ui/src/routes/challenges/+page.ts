import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getChallenges } from '$lib/api';
import { createMessage } from '$lib/response';
import type { ChallengeProps } from './+page.svelte';

export const load: PageLoad = async ({ url }): Promise<ChallengeProps> => {
    const participants = url.searchParams.get('participants');

    if (participants == null) {
        error(404, 'Participants query parameter is required');
    }

    const { ok, status, err, resp } = await getChallenges(participants);

    if (!ok) {
        error(status, createMessage(err));
    }

    return { isSender: participants === "sent", challengeList: resp || [] };
};
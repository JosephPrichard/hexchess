import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';
import type { ChallengeProps } from './+page.svelte';
import { createMessage } from '$lib/services/error';
import services from '$lib/api/services';

export const load: PageLoad = async ({ url, fetch }): Promise<ChallengeProps> => {
	const participants = url.searchParams.get('participants') || 'sent';

	const [data, err] = await services.getChallenges(participants, fetch);
	if (err) {
		error(err.status, createMessage(err));
	}

	return { participants, challengeList: data?.challengeList || [] };
};

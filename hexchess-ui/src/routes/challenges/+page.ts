import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';
import type { ChallengeProps } from './+page.svelte';
import { createMessage } from '$lib/error';
import { client } from '$lib/api';

export const load: PageLoad = async ({ url, fetch }): Promise<ChallengeProps> => {
	const participants = url.searchParams.get('participants') || 'sent';

	const resp = await client.GET("/views/challenges", { fetch });
	const status = resp.response.status;

	if (!resp.data) {
		error(status, createMessage(resp.error));
	}

	// const { ok, status, err, resp } = await unwrap(getChallenges(participants, fetch));
	// if (!ok || resp === undefined) {
	// 	error(status, createMessage(err));
	// }

	return { participants, challengeList: resp.data };
};

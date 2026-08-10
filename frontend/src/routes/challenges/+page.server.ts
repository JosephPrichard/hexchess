import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { handleError } from '$lib/utils/error';
import { services } from '$lib/api/services';
import type { Challenge } from '$lib/api/models';

export interface ChallengeProps {
	participants: string;
	challengeList: Challenge[];
}

export const load: PageServerLoad = async (event): Promise<ChallengeProps> => {
	const participants = event.url.searchParams.get('participants') || 'received';

	const [data, err] = await services.getChallenges(participants, event.fetch);
	if (err) {
		error(err.status, handleError(err));
	}

	return { participants, challengeList: data?.challengeList ?? [] };
};

import type { PageLoad } from './$types';
import type { ProfileProps } from './+page.svelte';
import { handleError } from '$lib/utils/error';
import { error } from '@sveltejs/kit';
import services from '$lib/api/services';

export const load: PageLoad = async ({ fetch }): Promise<ProfileProps> => {
	const [[countryData, countryErr], [profileData, profileErr]] = await Promise.all([services.getCountries(fetch), services.getProfile(fetch)]);

	if (countryErr) {
		error(countryErr.status, 'Unexpected error has occurred.');
	}
	if (profileErr || profileData === undefined) {
		error(profileErr?.status || 500, handleError(profileErr));
	}

	return { countryList: countryData ?? [], user: profileData };
};

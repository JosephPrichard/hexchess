import type { PageServerLoad } from './$types';
import { handleError } from '$lib/utils/error';
import { error } from '@sveltejs/kit';
import { services } from '$lib/api/services';
import type { User } from '$lib/api/models';

export interface ProfileProps {
	countryList: string[];
	user: User;
}

export const load: PageServerLoad = async (event): Promise<ProfileProps> => {
	const [[countryData, countryErr], [profileData, profileErr]] = await Promise.all([
		services.getCountries(), 
		services.getProfile(event.fetch)
	]);

	if (countryErr) {
		error(countryErr.status, 'Unexpected error has occurred.');
	}
	if (profileErr || profileData === undefined) {
		error(profileErr?.status || 500, handleError(profileErr));
	}

	return { countryList: countryData ?? [], user: profileData };
};

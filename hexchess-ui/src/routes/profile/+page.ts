import type { PageLoad } from './$types';
import type { ProfileProps } from './+page.svelte';
import { getCountries, getProfile, unwrap } from '$lib/api';
import { createMessage } from '$lib/error';
import { error } from '@sveltejs/kit';

export const load: PageLoad = async ({ fetch }): Promise<ProfileProps> => {
    const [countryList, profileResult] = await Promise.all([
        getCountries(fetch), 
        unwrap(getProfile(fetch))
    ]);

    if (!countryList) {
        error(500, "Unexpected error has occurred.");
    }

    if (!profileResult.ok || profileResult.resp === undefined) {
        error(profileResult.status, createMessage(profileResult.err));
    }

    return { countryList: countryList || [], user: profileResult.resp };
};
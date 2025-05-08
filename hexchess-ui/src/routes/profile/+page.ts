import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getSelfUser } from '$lib/api';
import { createMessage } from '$lib/response';
import type { ProfileProps } from './+page.svelte';

export const load: PageLoad = async (): Promise<ProfileProps> => {
    const { ok, status, err, resp } = await getSelfUser();

    if (!ok || !resp) {
        error(status, createMessage(err));
    }

    return { user: resp };
};
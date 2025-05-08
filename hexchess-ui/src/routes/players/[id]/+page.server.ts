import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getUserWithReplays } from '$lib/api';
import { createMessage } from '$lib/response';
import type { PlayerProps } from './+page.svelte';

export const load: PageServerLoad = async ({ params }): Promise<PlayerProps> => {
    const id = params.id;

    const { ok, status, err, resp } = await getUserWithReplays(id);

    if (!ok || !resp) {
        error(status, createMessage(err));
    }

    return { userWithReplays: resp };
};
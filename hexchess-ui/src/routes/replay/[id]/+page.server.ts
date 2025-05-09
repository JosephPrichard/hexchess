import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getInitialBoard, getReplay, unwrap } from '$lib/api';
import { createMessage } from '$lib/response';
import type { ReplayProps } from './+page.svelte';

export const load: PageServerLoad = async ({ params, setHeaders }): Promise<ReplayProps> => {
    const id = params.id;

    const [initialBoard, apiResultResp] = await Promise.all([getInitialBoard(), unwrap(getReplay(id))]);

    if (!initialBoard) {
        error(500, "Unexpected error has occurred.");
    }
    const { ok, status, err, resp } = apiResultResp;
    if (!ok || resp === undefined) {
        error(status, createMessage(err));
    }

    setHeaders({
        'cache-control': 'max-age=3600'
    });
    return { replay: resp, initialBoard };
};
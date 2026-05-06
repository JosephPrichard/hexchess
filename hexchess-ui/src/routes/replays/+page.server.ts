import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { makeMessage } from '$lib/utils/error';
import type { ReplaysProps } from './+page.svelte';
import services, {type ReplaysQuery} from '$lib/api/services';

export const load: PageServerLoad = async ({ url, setHeaders, fetch }): Promise<ReplaysProps> => {
    const replayQuery = Object.fromEntries(url.searchParams) as ReplaysQuery;

    const [data, err] = await services.getReplays(replayQuery, fetch);
    if (err || data === undefined) {
        error(err?.status || 500, makeMessage(err));
    }

    // setHeaders({
    // 	'cache-control': 'max-age=3600'
    // });
    return { replays: data?.replayList ?? [], query: replayQuery };
};

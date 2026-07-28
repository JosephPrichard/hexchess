import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { handleError } from '$lib/utils/error';
import type { ReplaysProps } from './+page.svelte';
import services from '$lib/api/services';
import {mapReplaysPropsToQuery, mapReplaysURLParamsToProps} from "./service";

export const load: PageServerLoad = async ({ url, setHeaders, fetch }): Promise<ReplaysProps> => {
    const searchProps = mapReplaysURLParamsToProps(url.searchParams);

    const [data, err] = await services.getReplays(mapReplaysPropsToQuery(searchProps), fetch);
    if (err || data === undefined) {
        error(err?.status || 500, handleError(err));
    }

    // setHeaders({
    // 	'cache-control': 'max-age=3600'
    // });
    return { replays: data?.replayList ?? [], search: searchProps };
};

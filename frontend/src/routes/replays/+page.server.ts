import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { handleError } from '$lib/utils/error';
import { services } from '$lib/api/services';
import {mapReplaysPropsToQuery, mapReplaysURLParamsToProps, type ReplaysSearchProps} from "./service";
import { env as publicEnv } from '$env/dynamic/public';
import type { Replay } from '$lib/api/models';

export interface ReplaysProps {
    replays: Replay[];
    search?: ReplaysSearchProps;
}

export const load: PageServerLoad = async (event): Promise<ReplaysProps> => {
    const searchProps = mapReplaysURLParamsToProps(event.url.searchParams);

    const [data, err] = await services.getReplays(mapReplaysPropsToQuery(searchProps));
    if (err || data === undefined) {
        error(err?.status || 500, handleError(err));
    }

    if (publicEnv.PUBLIC_ACTIVE_PROFILE == "prod") {
        event.setHeaders({ 'cache-control': 'max-age=3600' });
    }
    return { replays: data?.replayList ?? [], search: searchProps };
};

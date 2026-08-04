import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import {codes, handleError} from '$lib/utils/error';
import { services } from '$lib/api/services';
import { env as publicEnv } from '$env/dynamic/public';
import type { Replay } from '$lib/api/models';

export interface ReplayProps {
	replay: Replay;
}

export const load: PageServerLoad = async (event): Promise<ReplayProps> => {
	const id = event.params.id;

	const [data, err] = await services.getReplay(id, undefined);

	if (err?.message === codes.errorNotFoundReplay) {
		error(err?.status || 404, handleError(err));
	}
	if (err || data === undefined) {
		error(err?.status || 500, handleError(err));
	}

	if (publicEnv.PUBLIC_ACTIVE_PROFILE == "prod") {
		event.setHeaders({ 'cache-control': 'max-age=3600' });
	}
	return { replay: data.replay };
};

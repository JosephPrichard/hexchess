import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { services } from '$lib/api/services';
import { handleError } from '$lib/utils/error';
import { env as publicEnv } from '$env/dynamic/public';
import type { Persona } from '$lib/api/models';

export interface PlayerProps {
	fullUser: Persona;
}

export const load: PageServerLoad = async (event): Promise<PlayerProps> => {
	const id = event.params.id;

	const [data, err] = await services.getUser(id, true);

	if (err?.message === 'USER_NOT_FOUND') {
		error(err?.status || 404, handleError(err));
	}
	if (err || data === undefined) {
		error(err?.status || 500, handleError(err));
	}

	if (publicEnv.PUBLIC_ACTIVE_PROFILE == "prod") {
		event.setHeaders({ 'cache-control': 'max-age=300' });
	}
	return { fullUser: data };
};

import { logger } from '$lib/utils/logger';
import { building } from '$app/environment';
import globals from '$lib/api/globals';
import type { Handle } from '@sveltejs/kit';
import { env as privateEnv } from '$env/dynamic/private';

function isAllowedHeader(header: string) {
	switch (header.toLowerCase()) {
		case 'content-type':
			return true;
		default:
			return false;
	}
}

export const handle: Handle = ({ event, resolve }) => {
	return resolve(event, {
		filterSerializedResponseHeaders: isAllowedHeader
	});
};

// sets private environment variables into the server state store on startup
export const init = async () => {
	if (building) return;

	const backendBaseURL = privateEnv.INTERNAL_BACKEND_BASE_URL;
	
	logger.info("init sveltekit server", { 
		"INTERNAL_BACKEND_BASE_URL": backendBaseURL
	});

	globals.setInternalBackendBaseURL(backendBaseURL);
};
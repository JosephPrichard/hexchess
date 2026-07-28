import { logger } from '$lib/utils/logger';
import { json } from '@sveltejs/kit';

export function GET() {
    const resp = { status: 'ok', timestamp: new Date().toISOString() };
    // logger.info("healthcheck", resp)
    return json(resp);
}
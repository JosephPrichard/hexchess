import type { Handle } from '@sveltejs/kit';

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
        filterSerializedResponseHeaders: isAllowedHeader,
    });
}
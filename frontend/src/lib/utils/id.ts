export function generateRenderID(): string {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    return "id-" + Array.from(crypto.getRandomValues(new Uint8Array(24)))
        .map(b => chars[b % chars.length])
        .join('');
}
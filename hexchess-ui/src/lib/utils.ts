import type { Move, Session } from '$lib/models';

export function getClientSession(): Session | undefined {
    const value = ('; ' + document.cookie).split('; session=').pop();
    if (value === undefined) {
        return undefined;
    }
    const cookieStr = value.split(';')[0];
    return cookieStr !== '' ? JSON.parse(JSON.parse(cookieStr)) : undefined;
}

const symbols = ['p', 'p', 'n', 'n', 'b', 'b', 'r', 'r', 'q', 'q', 'k', 'k'];
const files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

export function stringOfMove(move: Move) {
    const symbol = symbols[move.piece] || '?';
    const toFile = files[move.to.file];
    const toRank = String(move.to.rank + 1);
    return symbol + toFile + toRank;
}

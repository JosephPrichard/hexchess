import { GameInput } from '$lib/pb/messages';
import type { MoveAction } from '$lib/components/chess/types';

export interface ConnectionState {
	tries: number
	ws?: WebSocket
	setAt?: Date
}

export function sendPingInput(connState: ConnectionState | undefined) {
	connState?.ws?.send?.(GameInput.toBinary({
		value: {
			oneofKind: 'ping',
			ping: {},
		}
	}));
}

export function sendMoveInput(connState: ConnectionState | undefined, {from, to, promotion: {kind}}: MoveAction) {
	connState?.ws?.send?.(GameInput.toBinary({
		value: {
			oneofKind: 'move',
			move: {
				move: { promotion: kind, fromFile: from.file, fromRank: from.rank, toFile: to.file, toRank: to.rank }
			}
		}
	}));
}

export function sendChatInput(connState: ConnectionState | undefined, chatText: string) {
	connState?.ws?.send?.(GameInput.toBinary({
		value: {
			oneofKind: 'chat',
			chat: { message: chatText }
		}
	}));
}

export function sendForfeitInput(connState: ConnectionState | undefined) {
	connState?.ws?.send?.(GameInput.toBinary({
		value: {
			oneofKind: 'forfeit',
			forfeit: {}
		}
	}));
}

export function sendUndoInput(connState: ConnectionState | undefined, kind: string) {
	connState?.ws?.send?.(GameInput.toBinary({
		value: {
			oneofKind: 'undo',
			undo: { kind }
		}
	}));
}
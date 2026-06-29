import { GameInput } from '$lib/pb/messages';
import type { MoveAction } from '$lib/components/types';
import type { Chat } from '$lib/api/models';

export function formatChats(chats: Chat[]): Chat[] {
	chats.sort((a, b) => a.sentAt.getTime() - b.sentAt.getTime());

	const usedIDs = new Set<string>();
	const indicesToRemove = [];
	for (const [index, chat] of chats.entries()) {
		if (usedIDs.has(chat.id)) {
			indicesToRemove.push(index);
		}
		usedIDs.add(chat.id);
	}

	for (const index of indicesToRemove) {
		chats.splice(index, 1);
	}

	return chats;
}

export interface ConnectionState {
	tries: number
	ws?: WebSocket
	setAt?: Date
}

export function sendPingInput(connState: ConnectionState | undefined) {
	const input = GameInput.toBinary({
		messageId: "",
		value: {
			oneofKind: 'ping',
			ping: {},
		}
	});
	connState?.ws?.send?.(toUint8Array(input));
}

export function sendMoveInput(connState: ConnectionState | undefined, {from, to, promotion: {kind}}: MoveAction) {
	const input = GameInput.toBinary({
		messageId: "",
		value: {
			oneofKind: 'move',
			move: {
				move: { promotion: kind, fromFile: from.file, fromRank: from.rank, toFile: to.file, toRank: to.rank }
			}
		}
	});
	connState?.ws?.send?.(toUint8Array(input));
}

export function sendChatInput(connState: ConnectionState | undefined, chatText: string) {
	const input = GameInput.toBinary({
		messageId: "",
		value: {
			oneofKind: 'chat',
			chat: { message: chatText }
		}
	});
	connState?.ws?.send?.(toUint8Array(input));
}

export function sendForfeitInput(connState: ConnectionState | undefined) {
	const input = GameInput.toBinary({
		messageId: "",
		value: {
			oneofKind: 'forfeit',
			forfeit: {}
		}
	});
	connState?.ws?.send?.(toUint8Array(input));
}

export function sendUndoInput(connState: ConnectionState | undefined, kind: string) {
	const input = GameInput.toBinary({
		messageId: "",
		value: {
			oneofKind: 'undo',
			undo: { kind }
		}
	});
	connState?.ws?.send?.(toUint8Array(input));
}

function toUint8Array(data: Uint8Array<ArrayBufferLike>): Uint8Array<ArrayBuffer> {
	// copies out to ensure an underlying array buffer is not a shared array buffer.
	const ab = new ArrayBuffer(data.byteLength);
	new Uint8Array(ab).set(data);
	return new Uint8Array(ab);
}
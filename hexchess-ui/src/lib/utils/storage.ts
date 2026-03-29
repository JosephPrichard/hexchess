import type { SessionModel } from '../api/models';

const SESSION_KEY = 'session';

interface LocalStorageRecord<T> {
	data: T;
	expiry: number;
}

export function getClientSession(): SessionModel | null {
	const recordStr = localStorage.getItem(SESSION_KEY);
	if (recordStr != null) {
		const record = JSON.parse(recordStr) as LocalStorageRecord<SessionModel>;
		const now = new Date().getTime();
		if (record.expiry < now) {
			localStorage.removeItem(SESSION_KEY);
			console.log(`Expired key=${SESSION_KEY} with value=${recordStr} from local storage at time=${now}`);
			return null;
		}
		return record.data;
	}
	return null;
}

export function setClientSession(client: SessionModel) {
	const record: LocalStorageRecord<SessionModel> = {
		data: client,
		expiry: new Date().getTime() + (client.ttlSecs ?? 0) * 1000
	};
	const recordStr = JSON.stringify(record);
	localStorage.setItem(SESSION_KEY, recordStr);

	// console.log(`Set key=${SESSION_KEY} to value=${recordStr} to local storage`);
}

export function updateClientSession(newClient: SessionModel | null) {
	if (newClient) {
		const client = getClientSession();
		if (client != null) {
			newClient = { ...newClient, ttlSecs: newClient.ttlSecs || client.ttlSecs };
			setClientSession(newClient);
		}
	}
}

export function clearClientSession() {
	console.log('Clearing client session');
	localStorage.removeItem(SESSION_KEY);
}

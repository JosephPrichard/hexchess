import { getContext, setContext } from 'svelte';
import type { ChallengeModel } from '../api/model';
import type { Writable } from 'svelte/store';

export interface NotificationValue {
	isSuccess: boolean;
	duration?: number;
}

type TextValue = NotificationValue & {
	type: 'string';
	message: string;
};

type ChallengeValue = NotificationValue & {
	type: 'challenge';
	message: ChallengeModel;
};

export type NotificationData = TextValue | ChallengeValue;

interface NotificationsContext {
	addNotification: (data: NotificationData) => void;
	deleteNotification: (index: number) => void;
	counts: Writable<{ usersCount: number; gameCounts: number; }>;
}

export function getNotificationsContext() {
	return getContext('notifications') as NotificationsContext;
}

export function setNotificationsContext(ctx: NotificationsContext) {
	return setContext('notifications', ctx);
}

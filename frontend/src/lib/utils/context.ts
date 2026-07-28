import { getContext, setContext } from 'svelte';
import type { Challenge, ServiceResponse } from '../api/models';

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
	message: Challenge;
};

export type NotificationData = TextValue | ChallengeValue;

export type AddNotification = (data: NotificationData) => void;
export type AddErrorNotification = (message: string | ServiceResponse | undefined) => void;

interface NotificationsContext {
	addNotification: AddNotification;
	addErrorNotification: AddErrorNotification;
	deleteNotification: (index: number) => void;
}

export function getNotificationsContext() {
	return getContext('notifications') as NotificationsContext;
}

export function setNotificationsContext(ctx: NotificationsContext) {
	return setContext('notifications', ctx);
}

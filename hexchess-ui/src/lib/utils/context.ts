import { getContext, setContext } from 'svelte';
import type { ChallengeModel, ServiceModel } from '../api/models';
import type { Writable } from 'svelte/store';
import { makeMessage } from '$lib/utils/error';

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

export type AddNotification = (data: NotificationData) => void;
export type AddErrorNotification = (message: string | ServiceModel | undefined) => void;

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

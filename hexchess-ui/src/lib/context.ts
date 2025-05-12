import { getContext, setContext } from 'svelte';

export interface NotificationData {
    message: string;
    isSuccess: boolean;
}

interface NotificationsContext {
    addNotification: (data: NotificationData, expire: number) => void;
    deleteNotification: (index: number) => void;
}

export function getNotificationsContext() {
    return getContext("notifications") as NotificationsContext;
}

export function setNotificationsContext(ctx: NotificationsContext) {
    return setContext("notifications", ctx);
}
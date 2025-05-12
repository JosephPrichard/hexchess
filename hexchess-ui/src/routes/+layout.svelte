<script lang="ts">
    import type { LayoutProps } from '../../.svelte-kit/types/src/routes/$types';
    import { type NotificationData, setNotificationsContext } from '$lib/context';

    const { children }: LayoutProps = $props();

    const notifications: (NotificationData | undefined)[] = $state([]);
    const timeouts = new Map<number, number>();
    let index = 0;

    function deleteNotification(index: number) {
        notifications[index] = undefined;
        const timeout = timeouts.get(index);
        if (timeout) {
            clearTimeout(timeout);
        }
        timeouts.delete(index);
    }

    function addNotification(notification: NotificationData, expire: number) {
        const i = index++;
        notifications[i] = notification;
        timeouts.set(i, setTimeout(() => deleteNotification(i), expire));
    }

    setNotificationsContext({ addNotification, deleteNotification })
</script>

<div class="bottom-right-anchor" style="width: 350px">
    {#each notifications as notification, i (i)}
        {#if notification}
            <div class={`notification ${notification.isSuccess ? 'notification-green' : 'notification-red'}`}>
                <div class="notification-text">
                    {notification.message}
                </div>
                <div class="notification-space"></div>
                <button class="notification-x" onclick={() => deleteNotification(i)}>
                    &#10006;
                </button>
            </div>
        {/if}
    {/each}
</div>
{@render children()}
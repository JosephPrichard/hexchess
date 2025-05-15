<script lang="ts">
    import type { LayoutProps } from '../../.svelte-kit/types/src/routes/$types';
    import { type NotificationData, type NotificationValue, setNotificationsContext } from '$lib/context';
    import { onMount } from 'svelte';
    import type { ChallengeMsg } from '$lib/models';
    import { baseURL } from '$lib/api';
    import { createMessage } from '$lib/error';

    const { children }: LayoutProps = $props();

    const notifications: (NotificationData | undefined)[] = $state([]);
    const timeouts: Record<number, number> = {};
    let index = 0;
    let sse: EventSource | undefined = undefined;

    function deleteNotification(index: number) {
        notifications[index] = undefined;
        const timeout = timeouts[index];
        if (timeout) {
            clearTimeout(timeout);
        }
        delete timeouts[index];
    }

    function addNotification(data: NotificationData) {
        const i = index++;
        notifications[i] = data;
        timeouts[i] = setTimeout(() => deleteNotification(i), data.duration);
    }

    function connectUserEvents() {
        sse = new EventSource(`${baseURL}/events/user`, {
            withCredentials: true,
        });
        sse.addEventListener("meta", (event) => {
            console.log("Sse:", createMessage(event.data));
        });
        sse.addEventListener("challenge", (event) => {
            const data: ChallengeMsg = JSON.parse(event.data);
            console.log("Sse:", data);
            addNotification({ type: 'challenge', message: data, isSuccess: true, duration: 10000 });
        });
    }

    onMount(() => {
        connectUserEvents();
        return () => {
            if (sse) {
                sse.close();
            }
        }
    });

    setNotificationsContext({ addNotification, deleteNotification });
</script>

<div class="bottom-right-anchor notifications-box" style="width: 350px">
    {#each notifications as notification, i (i)}
        {#if notification}
            <div class="notification">
                <div class="notification-border {notification.isSuccess ? 'notification-green' : 'notification-red'}"></div>
                <div class="notification-body">
                    <div class="notification-text">
                        {#if notification.type === 'string'}
                            {notification.message}
                        {:else if notification.type === 'challenge'}
                            {@const challenge = notification.message}
                            Player <a href="/players/{challenge.challengeeId}"> {challenge.challengeeName} </a>
                            has challenged you to a <a href="/challenges?participants=received&id={challenge.challengerId}"> game </a>
                        {/if}
                    </div>
                    <div class="notification-space"></div>
                    <button class="notification-x" onclick={() => deleteNotification(i)}>
                        &#10006;
                    </button>
                </div>
            </div>
        {/if}
    {/each}
</div>
{@render children()}
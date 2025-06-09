<script lang="ts">
	import '../css/index.css';
	import type { LayoutProps } from '../../.svelte-kit/types/src/routes/$types';
	import { type NotificationData, setNotificationsContext } from '$lib/utils/context';
	import { onMount } from 'svelte';
	import { createMessage } from '$lib/utils/error';
	import { clearClientSession, updateClientSession } from '$lib/utils/storage';
	import services, { baseURL } from '$lib/api/services';
	import type { ChallengeModel } from '$lib/api/model';

	const { children }: LayoutProps = $props();

	const notifications: (NotificationData | undefined)[] = $state([]);
	const timeouts: Record<number, ReturnType<typeof setTimeout>> = {};
	let index = 0;
	let sse: EventSource | undefined = undefined;
	let refreshInterval: ReturnType<typeof setInterval> | undefined = undefined;

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
		sse = new EventSource(`${baseURL}/events/user/subscriptions`, {
			withCredentials: true
		});
		sse.addEventListener('meta', (event) => {
			console.log('Sse:', createMessage(event.data));
		});
		sse.addEventListener('challenge', (event) => {
			const data: ChallengeModel = JSON.parse(event.data);
			console.log('Sse:', data);
			addNotification({ type: 'challenge', message: data, isSuccess: true, duration: 150000 });
		});
	}

	async function refreshSession() {
		const [data, _] = await services.postRefresh();
		if (data) {
			if (data.session) {
				updateClientSession(data.session);
			} else {
				clearClientSession();
			}
		}
	}

	onMount(() => {
		connectUserEvents();
		refreshSession();
		refreshInterval = setInterval(async () => refreshSession(), 900000); // 15 minutes
		return () => {
			if (refreshInterval) {
				clearInterval(refreshInterval);
			}
			if (sse) {
				sse.close();
			}
		};
	});

	setNotificationsContext({ addNotification, deleteNotification });
</script>

<div class="bottom-right-anchor notifications-box">
	{#each notifications as notification, i (i)}
		{#if notification}
			<div class="notification">
				<div class="notification-border" class:notification-green={notification.isSuccess} class:notification-red={!notification.isSuccess}>
				</div>
				<div class="notification-body">
					<div class="notification-text">
						{#if notification.type === 'string'}
							{notification.message}
						{:else if notification.type === 'challenge'}
							{@const challenge = notification.message}
							Player <a href="/players/{challenge.challengeeId}"> {challenge.challengeeName} </a>
							has challenged you to a <a href="/challenges?participants=received"> game </a>
						{/if}
					</div>
					<div class="notification-space"></div>
					<button class="notification-x" onclick={() => deleteNotification(i)}> &#10006; </button>
				</div>
			</div>
		{/if}
	{/each}
</div>

{@render children()}

<style>
    .notifications-box {
        width: 350px;
        z-index: 10000;
    }

    .notification-border {
        border-radius: 2px;
        height: 3px;
        width: 100%;
    }

    .notification-green {
        background: #2ea44f;
    }

    .notification-red {
        background: crimson;
    }

    .notification {
        display: flex;
        flex-direction: column;
        z-index: 10000;
        margin: 20px;
        color: white;
        border-radius: 2px;
        background-color: rgb(43, 43, 43);
    }

    .notification-body {
        display: flex;
        flex-direction: row;
        padding: 20px 30px;
    }

    .notification-text {
        flex: 0.9;
    }

    .notification-space {
        flex: 0.05;
    }

    .notification-x {
        all: unset;
        flex: 0.05;
        float: right;
        margin-right: auto;
        cursor: pointer;
    }
</style>
<script lang="ts">
	import '../css/index.css';
	import type { LayoutProps } from '../../.svelte-kit/types/src/routes/$types';
	import { type AddNotification, type NotificationData, setNotificationsContext } from '$lib/utils/context';
	import { onMount } from 'svelte';
	import { clearClientSession, updateClientSession } from '$lib/utils/storage';
	import services, { backendBaseURL } from '$lib/api/services';
	import type { Challenge, ServiceResponse } from '$lib/api/models';
	import { fade } from 'svelte/transition';
	import { handleError } from '$lib/utils/error';

	const { children }: LayoutProps = $props();

	interface Notification {
		data: NotificationData;
		index: number;
	}

	let notifications: Record<number, Notification> = $state({});

	const timeouts: Record<number, ReturnType<typeof setTimeout>> = {};
	let index = 0;
	let userSse: EventSource | undefined = undefined;
	let activeSse: EventSource | undefined = undefined;
	let refreshInterval: ReturnType<typeof setInterval> | undefined = undefined;

	function deleteNotification(index: number) {
		delete notifications[index];
		const timeout = timeouts[index];
		if (timeout) {
			clearTimeout(timeout);
		}
		delete timeouts[index];
	}

	function addNotification(data: NotificationData) {
		const i = index++;
		notifications[i] = {data, index: i};
		timeouts[i] = setTimeout(() => deleteNotification(i), data.duration || 3000);
	}

	function addErrorNotification(message: string | ServiceResponse | undefined) {
		if (Object.values(notifications).length >= 5) return;
		
		addNotification({
			isSuccess: false,
			message: handleError(message),
			type: 'string'
		});
	}

	function connectUserEvents() {
		userSse = new EventSource(`${backendBaseURL()}/events/user`, {
			mode: 'cors',
			withCredentials: true
		});
		userSse.addEventListener('userEvents', (event) => {
			logger.info('Sse: USER_EVENTS userEvents', event.data);
			const data: Challenge = JSON.parse(event.data);
			addNotification({ type: 'challenge', message: data, isSuccess: true, duration: 6000 });
		});
	}

	function connectActiveConn() {
		activeSse = new EventSource(`${backendBaseURL()}/events/active`, {
			mode: 'cors',
			withCredentials: true
		});
	}

	async function refreshSession(retries?: number) {
		const [data, err] = await services.postRefreshSession();
		if (data) {
			if (data.session) {
				updateClientSession(data.session);
			} else {
				clearClientSession();
			}
		} else {
			logger.error('error refreshing session', err);
			const retry = retries || 1;
			setTimeout(() => refreshSession(retry + 1), 50 * Math.pow(2, retry));
		}
	}

	onMount(() => {
		connectActiveConn();
		connectUserEvents();
		refreshSession();
		refreshInterval = setInterval(async () => refreshSession(), 900000); // 15 minutes
		return () => {
			if (refreshInterval) clearInterval(refreshInterval);
			if (userSse) userSse.close();
			if (activeSse) activeSse.close();
		};
	});

	setNotificationsContext({ addNotification, addErrorNotification, deleteNotification });
</script>

<svelte:head>
  <script src="https://accounts.google.com/gsi/client" async defer></script>
</svelte:head>
<div class="bottom-right-anchor notifications-box">
	{#each Object.values(notifications) as {data: notification, index}}
		<div in:fade={{ duration: 300, delay: 0 }} out:fade={{ duration: 300, delay: 0 }}
			 class="notification"
			 class:notification-green={notification?.isSuccess}
			 class:notification-red={!notification?.isSuccess}
		>
			<div class="notification-body">
				<div class="notification-text">
					{#if notification?.type === 'string'}
						{notification?.message}
					{:else if notification?.type === 'challenge'}
						{@const challenge = notification?.message}
						Player <a href="/players/{challenge.challengerId}"> {challenge.challengerName} </a>
						has challenged you to a <a href="/challenges?participants=received"> game </a>
					{/if}
				</div>
				<div class="notification-space"></div>
				<button class="notification-x" onclick={() => deleteNotification(index)}> &#10006; </button>
			</div>
		</div>
	{/each}
</div>

{@render children()}

<footer class="footer">
	<p>
		&copy; 2025 Hesketh Prichard, Joseph. All rights reserved.
	</p>
	<p>
		<a href="https://github.com/JosephPrichard/hexchess" target="_blank" rel="noopener noreferrer">Source code</a> |
		<a href="/terms-and-conditions" target="_blank">Terms &amp; Conditions</a>
	</p>
</footer>

<style>
	.footer {
        text-align: center;
		padding: 20px;
		font-size: 14px;
		margin-top: 50px;
		color: rgb(160, 160, 160);
	}

    .notifications-box {
        width: 350px;
        z-index: 15;
    }

    .notification-green {
        background-color: #2ea44f;
    }

    .notification-red {
        background-color: crimson;
    }

    .notification {
		border-radius: 5px;
        display: flex;
        flex-direction: column;
        z-index: 10;
        margin: 20px;
        color: white;
        /*background-color: rgb(43, 43, 43);*/
        transition:
			opacity 0.2s ease,
			transform ease;
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
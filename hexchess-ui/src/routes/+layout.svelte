<script lang="ts">
	import '../css/index.css';
	import type { LayoutProps } from '../../.svelte-kit/types/src/routes/$types';
	import { type NotificationData, setNotificationsContext } from '$lib/services/context';
	import { onMount } from 'svelte';
	import { clearClientSession, updateClientSession } from '$lib/services/storage';
	import services, { baseURL } from '$lib/api/services';
	import type { ChallengeModel } from '$lib/api/model';
	import { writable } from 'svelte/store';
	import { fade } from 'svelte/transition';

	const { children }: LayoutProps = $props();

	let notifications: Record<number, NotificationData> = $state({});
	let counts = writable({ usersCount: 0, gameCounts: 0 });

	const timeouts: Record<number, ReturnType<typeof setTimeout>> = {};
	let index = 0;
	let userSse: EventSource | undefined = undefined;
	let countSse: EventSource | undefined = undefined;
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
		notifications[i] = data;
		timeouts[i] = setTimeout(() => deleteNotification(i), data.duration || 3000);
	}

	function connectUserEvents() {
		userSse = new EventSource(`${baseURL()}/events/user`, {
			mode: 'cors',
			withCredentials: true
		});
		userSse.addEventListener('meta', (event) => {
			console.log('Sse: /events/user meta', event.data);
		});
		userSse.addEventListener('userEvents', (event) => {
			console.log('Sse: /events/user userEvents', event.data);
			const data: ChallengeModel = JSON.parse(event.data);
			addNotification({ type: 'challenge', message: data, isSuccess: true });
		});
	}

	function connectCountEvents() {
		countSse = new EventSource(`${baseURL()}/events/count`);
		countSse.addEventListener('meta', (event) => {
			console.log('Sse: /events/count meta', event.data);
		});
		countSse.addEventListener('activeCountEvents', (event) => {
			console.log('Sse: /events/count activeCountEvents', event.data);

			const count = Number(event.data);
			if (!isNaN(count)) {
				counts.update((value) => ({ ...value, usersCount: count }));
			}
		});
		countSse.addEventListener('gameCountEvents', (event) => {
			console.log('Sse: /events/count gameCountEvents', event.data);

			const count = Number(event.data);
			if (!isNaN(count)) {
				counts.update((value) => ({ ...value, gameCounts: count }));
			}
		});
	}

	async function refreshSession() {
		const [data, _] = await services.postRefreshSession();
		if (data) {
			if (data.session) {
				updateClientSession(data.session);
			} else {
				clearClientSession();
			}
		}
	}

	onMount(() => {
		connectCountEvents();
		connectUserEvents();
		refreshSession();
		refreshInterval = setInterval(async () => refreshSession(), 900000); // 15 minutes
		return () => {
			if (refreshInterval) {
				clearInterval(refreshInterval);
			}
			if (userSse) {
				userSse.close();
			}
			if (countSse) {
				countSse.close();
			}
		};
	});

	setNotificationsContext({ addNotification, deleteNotification, counts });
</script>

<div class="bottom-right-anchor notifications-box">
	{#each Object.values(notifications) as notification, i (i)}
		<div in:fade={{ duration: 300, delay: 0 }} out:fade={{ duration: 300, delay: 0 }} class="notification">
			<div class="notification-border" class:notification-green={notification?.isSuccess} class:notification-red={!notification?.isSuccess}>
			</div>
			<div class="notification-body">
				<div class="notification-text">
					{#if notification?.type === 'string'}
						{notification?.message}
					{:else if notification?.type === 'challenge'}
						{@const challenge = notification?.message}
						Player <a href="/players/{challenge.challengeeId}"> {challenge.challengeeName} </a>
						has challenged you to a <a href="/challenges?participants=received"> game </a>
					{/if}
				</div>
				<div class="notification-space"></div>
				<button class="notification-x" onclick={() => deleteNotification(i)}> &#10006; </button>
			</div>
		</div>
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
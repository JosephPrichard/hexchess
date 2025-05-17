<script lang="ts">
	import type { LayoutProps } from '../../.svelte-kit/types/src/routes/$types';
	import { type NotificationData, setNotificationsContext } from '$lib/context';
	import { onMount } from 'svelte';
	import type { ChallengeMsg } from '$lib/models';
	import { baseURL, postRefresh, unwrap } from '$lib/api';
	import { createMessage } from '$lib/error';
	import { clearClientSession, updateClientSession } from '$lib/local';

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
			const data: ChallengeMsg = JSON.parse(event.data);
			console.log('Sse:', data);
			addNotification({ type: 'challenge', message: data, isSuccess: true, duration: 150000 });
		});
	}

	async function refreshSession() {
		let { ok, resp } = await unwrap(postRefresh());
		if (ok) {
			console.log("Refresh session", resp);
			if (resp && resp.session) {
				updateClientSession(resp.session);
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

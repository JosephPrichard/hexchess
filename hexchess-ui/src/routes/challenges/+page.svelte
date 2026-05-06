<script lang="ts">
	import { getNotificationsContext } from '$lib/utils/context';
	import {type Action, type ChallengeModel, GameModeNameMap, type SessionModel} from '$lib/api/models';
	import services from '$lib/api/services';
	import { formatRelativeTime } from '$lib/utils/format';
	import Banner from '$lib/Banner.svelte';
	import ProfilePic from '$lib/components/ProfilePic.svelte';
	import ChallengeIcon from "$lib/icons/ChallengeIcon.svelte";
	import {onMount} from "svelte";
	import {getClientSession} from "$lib/utils/storage";

	export interface ChallengeProps {
		participants: string;
		challengeList: ChallengeModel[];
	}

	const { data: props }: { data: ChallengeProps } = $props();
	const isSender = $derived(props.participants === 'sent');

	interface ChallengeState {
		challenge: ChallengeModel;
		isLoading: {
			delete: boolean;
			accept: boolean;
			reject: boolean;
		};
	}

	let challengeList: ChallengeState[] = $state([]);
	let client: SessionModel | null = $state(null);

	$effect(() => {
		challengeList = props.challengeList.map((e) => ({
			challenge: e,
			isLoading: {
				delete: false,
				accept: false,
				reject: false
			}
		}));
	});

	const { addNotification, addErrorNotification } = getNotificationsContext();

	function formatSuccessMessage(challenge: ChallengeModel, action: Action) {
		let message: string | undefined = undefined;
		switch (action) {
			case 'delete':
				message = `Deleted the challenge against ${challenge.challengeeName}.`;
				break;
			case 'accept':
				message = `Accepted the challenge from ${challenge.challengerName}!`;
				break;
			case 'reject':
				message = `Rejected the challenge from ${challenge.challengerName}.`;
				break;
		}
		return message;
	}

	async function onUpdateChallenge(challenge: ChallengeModel, index: number, action: Action) {
		challengeList[index].isLoading[action] = true;

		const [data, err] = await services.postUpdateChallenge(challenge.challengerId, challenge.challengeeId, action);
		if (data) {
			const message = formatSuccessMessage(challenge, action);
			addNotification({
				type: 'string',
				message: message || 'An unexpected error has occurred',
				isSuccess: true,
				duration: 3000
			});
			challengeList.splice(index, 1);
		} else {
			addErrorNotification(err);
			challengeList[index].isLoading[action] = false;
		}
	}
	export type ActiveState = "active" | "inactive" | "loading";

	let activeState: Record<string, ActiveState> = $state({});

	$effect(() => {
		const client = getClientSession();

		const userIds = challengeList.map(e => e.challenge.challengerId)
				.concat(challengeList.map(e => e.challenge.challengeeId));

		for (const userId of userIds) {
			if (userId == client?.id) continue;
			if (!activeState[userId]) {
				activeState[userId] = "loading";
				services.getIsUserActive(userId).then(([data, err]) => {
					if (data) {
						activeState[userId] = data.isUserActive ? "active" : "inactive";
					} else {
						console.error(`Error fetching active state for user ${userId}`, err)
					}
				});
			}
		}
	});
</script>

<svelte:head>
	<title>Challenges - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-vertical-container challenge-bottom">
	<div class="challenge-wrapper">
		<div class="tabs-group">
			<a class="tab" class:tab-selected={!isSender} href="?participants=received"> Received </a>
			<a class="tab" class:tab-selected={isSender} href="?participants=sent"> Sent </a>
		</div>
		{#if challengeList.length > 0}
			<div class="challenge-panel">
				{#each challengeList as { challenge, isLoading }, index (index)}
					{@const challengerActiveState = activeState[challenge.challengerId]}
					{@const challengeeActiveState = activeState[challenge.challengeeId]}
					<div id="{challenge.challengeeId}+{challenge.challengerId}" class="challenge-box">
						<div class="pvp-wrapper">
							<div class="player-points-wrapper">
								<span class="pfp-wrapper">
									<ProfilePic userId={challenge.challengerId} size={45}/>
								</span>
								<a href="/players/{challenge.challengerId}" class="text-ul bold-link">
									{challenge.challengerName}
								</a>
								{#if !isSender}
									<img class="flag" src="/flags/{challenge.challengerCountry}.png" alt="" />
									<b>({Math.round(challenge.challengerElo)})</b>
								{/if}
								<div class:inactive-indicator={challengerActiveState === "inactive"}
									 class:active-indicator={challengerActiveState === "active"}></div>
							</div>
							<div class="vs-wrapper">
								<ChallengeIcon/>
							</div>
							<div class="player-points-wrapper">
								<span class="pfp-wrapper">
									<ProfilePic userId={challenge.challengeeId} size={45}/>
								</span>
								<a href="/players/{challenge.challengeeId}" class="text-ul bold-link">
									{challenge.challengeeName}
								</a>
								{#if isSender}
									<img class="flag" src="/flags/{challenge.challengeeCountry}.png" alt="" />
									<b>({Math.round(challenge.challengeeElo)})</b>
								{/if}
								<div class:inactive-indicator={challengeeActiveState === "inactive"}
									 class:active-indicator={challengeeActiveState === "active"}></div>
							</div>
						</div>
						<div class="buttons-wrapper">
							<div>
								<b>{GameModeNameMap[challenge.mode]}</b>
							</div>
							<div class="times-wrapper">
								<div style="margin-bottom: 6px">
									Sent {formatRelativeTime(challenge.madeOn)}
								</div>
								<div style="margin-bottom: 6px">
									Expires {formatRelativeTime(challenge.expiresOn)}
								</div>
							</div>
							{#if isSender}
								<button
									class="button-small button-small-red button-challenge"
									onclick={() => onUpdateChallenge(challenge, index, 'delete')}
								>
									{#if isLoading.delete}
										<div class="loader"></div>
									{:else}
										Delete
									{/if}
								</button>
							{:else}
								<button
									class="button-small button-small-green button-challenge"
									onclick={() => onUpdateChallenge(challenge, index, 'accept')}
								>
									{#if isLoading.accept}
										<div class="loader"></div>
									{:else}
										Accept
									{/if}
								</button>
								<button
									class="button-small button-small-red button-challenge"
									onclick={() => onUpdateChallenge(challenge, index, 'reject')}
								>
									{#if isLoading.reject}
										<div class="loader"></div>
									{:else}
										Reject
									{/if}
								</button>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{:else}
			<div class="color-wrapper">
				{#if isSender}
					No challenges have been sent
				{:else}
					No challenges have been received
				{/if}
			</div>
		{/if}
	</div>
</div>

<style>
	.active-indicator {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: #629924;
		margin-left: 5px;
	}

	.inactive-indicator {
		width: 10px; height: 10px;
		border-radius: 50%;
		border: 2.5px solid #888;
		position: relative;
		overflow: hidden;
		margin-left: 5px;
	}
	.inactive-indicator::before {
		content: '';
		position: absolute;
		top: 0; right: 0; bottom: 0;
		width: 50%;
		background: #888;
	}

	.pvp-wrapper {
        margin-bottom: 6px;
		flex: 0.5;
	}

	.buttons-wrapper {
		margin-left: 25px;
		flex: 0.5;
	}

	.vs-wrapper {
		width: 100%;
		text-align: center;
	}

	.player-points-wrapper {
		display: flex;
		align-items: center;
	}

	.pfp-wrapper {
		position: relative;
		margin-right: 8px;
	}

	.challenge-bottom {
		margin-bottom: 100px;
	}

	.challenge-wrapper {
		width: 600px;
	}

    .challenge-panel {
        border-radius: 5px;
        background-color: rgb(42, 42, 42);
        box-shadow: rgba(0, 0, 0, 0.16) 0 1px 2px;
    }

    .challenge-box {
        width: calc(100% - 60px);
        padding: 15px 30px;
        border-radius: 3px;
        background-color: rgb(42, 42, 42);
        display: flex;
        flex-direction: row;
        z-index: 2;
    }

	.challenge-box:hover {
		background-color: rgba(43, 71, 94, 0.5);
	}

	.times-wrapper {
		margin-top: 10px;
		margin-bottom: 10px;
	}

	.button-challenge {
        margin-right: 5px;
		margin-bottom: 10px;
	}
</style>
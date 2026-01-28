<script lang="ts">
	import { makeMessage } from '$lib/utils/error';
	import { formatJoinedOn } from '$lib/utils/format';
	import { getNotificationsContext } from '$lib/utils/context';
	import type { Action, ChallengeModel, UserModel } from '$lib/api/models';
	import services from '$lib/api/services';
	import { formatRelativeTime, getWinrateClass } from '$lib/utils/format';
	import Banner from '$lib/Banner.svelte';
	import ProfilePic from '$lib/components/user/ProfilePic.svelte';

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

	const { addNotification } = getNotificationsContext();

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
			addNotification({ type: 'string', message: makeMessage(err), isSuccess: false });
			challengeList[index].isLoading[action] = false;
		}
	}
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
					<div id="{challenge.challengeeId}+{challenge.challengerId}" class="challenge-box">
						<div>
							<div class="pvp-wrapper">
								<div class="player-points-wrapper">
									<span class="pfp-wrapper"><ProfilePic userId={challenge.challengerId} size={45}/></span>
									<a href="/players/{challenge.challengerId}" class="text-ul bold-link">
										{challenge.challengerName}
									</a>
									{#if !isSender}
										<img class="flag" src="/flags/{challenge.challengerCountry}.png" alt="" />
										<b>({Math.round(challenge.challengerElo)})</b>
									{/if}
								</div>
								<div class="vs-wrapper">
									V.S.
								</div>
								<div class="player-points-wrapper">
									<span class="pfp-wrapper"><ProfilePic userId={challenge.challengeeId} size={45}/></span>
									<a href="/players/{challenge.challengeeId}" class="text-ul bold-link">
										{challenge.challengeeName}
									</a>
									{#if isSender}
										<img class="flag" src="/flags/{challenge.challengeeCountry}.png" alt="" />
										<b>({Math.round(challenge.challengeeElo)})</b>
									{/if}
								</div>
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
						<div class="center-relative">
							<div class="vertical-align">

							</div>
						</div>
					</div>
					{#if index !== challengeList.length - 1}
						<div class="challenge-border"></div>
					{/if}
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
	.challenge-border {
		height: 1px;
		border-bottom: 1px solid rgb(62,62,62);
	}

	.pvp-wrapper {
        margin-bottom: 6px;
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
        padding: 15px 30px;
        border-radius: 5px;
        background-color: rgb(42, 42, 42);
        box-shadow: rgba(0, 0, 0, 0.16) 0 1px 2px;
    }

    .challenge-box {
        width: 100%;
        padding: 15px 0;
        border-radius: 3px;
        background-color: rgb(42, 42, 42);
        display: flex;
        flex-direction: row;
        z-index: 2;
    }

	.times-wrapper {
		margin-top: 30px;
		margin-bottom: 30px;
	}

	.button-challenge {
        margin-right: 5px;
		margin-bottom: 10px;
	}
</style>
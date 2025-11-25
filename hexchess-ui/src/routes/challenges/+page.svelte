<script lang="ts">
	import Banner from '$lib/Banner.svelte';
	import { makeMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import type { Action, ChallengeModel } from '$lib/api/model';
	import services from '$lib/api/services';

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
			const message = makeMessage(err);
			addNotification({ type: 'string', message, isSuccess: false });
			challengeList[index].isLoading[action] = false;
		}
	}
</script>

<svelte:head>
	<title>Challenges - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container challenge-bottom">
	<div class="challenge-wrapper">
		<div class="tabs-group">
			<a class="tab" class:tab-selected={!isSender} href="?participants=received"> Received </a>
			<a class="tab" class:tab-selected={isSender} href="?participants=sent"> Sent </a>
		</div>
		{#if challengeList.length > 0}
			<div class="challenge-list">
				{#each challengeList as { challenge, isLoading }, index (index)}
					<div id="{challenge.challengeeId}+{challenge.challengerId}" class="challenge-box">
						<div>
							<div style="margin-bottom: 6px">
								<a href="/players/{challenge.challengerId}" class="text-ul bold-link">{challenge.challengerName}</a>
								{#if !isSender}
									<img class="flag" src="/flags/{challenge.challengerCountry}.png" alt="" />
									<b>({challenge.challengerElo})</b>
								{/if}
								vs
								<a href="/players/{challenge.challengeeId}" class="text-ul bold-link">{challenge.challengeeName}</a>
								{#if isSender}
									<img class="flag" src="/flags/{challenge.challengeeCountry}.png" alt="" />
									<b>({challenge.challengeeElo})</b>
								{/if}
							</div>
							<div style="margin-bottom: 6px">
								Sent {challenge.madeAgo}
							</div>
							<div style="margin-bottom: 6px">
								Expires {challenge.expiresIn}
							</div>
						</div>
						<div class="center-relative">
							<div class="vertical-align">
								{#if isSender}
									<button
										class="button-small button-small-red"
										onclick={() => onUpdateChallenge(challenge, index, 'delete')}
										style="margin-left: 5px"
									>
										{#if isLoading.delete}
											<div class="loader"></div>
										{:else}
											Delete
										{/if}
									</button>
								{:else}
									<button
										class="button-small button-small-green"
										onclick={() => onUpdateChallenge(challenge, index, 'accept')}
										style="margin-left: 5px"
									>
										{#if isLoading.accept}
											<div class="loader"></div>
										{:else}
											Accept
										{/if}
									</button>
									<button
										class="button-small button-small-red"
										onclick={() => onUpdateChallenge(challenge, index, 'reject')}
										style="margin-left: 5px"
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
	.challenge-bottom {
		margin-bottom: 100px;
	}

	.challenge-wrapper {
		width: 600px;
	}

    .challenge-list {
        padding: 15px 30px;
        border-radius: 5px;
        background-color: rgb(42, 42, 42);
        /*box-shadow: rgba(0, 0, 0, 0.24) 0 2px 4px;*/
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
</style>
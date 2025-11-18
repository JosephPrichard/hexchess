<script lang="ts">
	import Banner from '$lib/Banner.svelte';
	import CreateGame from '$lib/components/modals/CreateGame.svelte';
	import { createMessage } from '$lib/services/error';
	import { getNotificationsContext } from '$lib/services/context';
	import { goto } from '$app/navigation';
	import type { ChessModel, ColorSelect, SessionModel, TimeControl } from '$lib/api/model';
	import services from '$lib/api/services';
	import { formatTimeControl } from '$lib/services/format';
	import { onMount } from 'svelte';
	import { getClientSession } from '$lib/services/storage';
	import { chessRowHeight, maxChessRows } from '$lib/services/render';

	export interface IndexProps {
		chessList: ChessModel[];
		selfChessList: ChessModel[];
	}

	const { data: props }: { data: IndexProps } = $props();

	let showSelf = $state(false);
	let showCreateModal = $state(false);
	let userCounts = $state(0);
	let gameCounts = $state(0);
	let client: SessionModel | null = $state(null);

	const chessList = $derived(showSelf ? props.selfChessList : props.chessList);
	const bottomPadding = $derived((chessRowHeight * maxChessRows) - (chessRowHeight * chessList.length));

	const { addNotification, counts } = getNotificationsContext();

	counts.subscribe((value) => {
		userCounts = value.usersCount;
		gameCounts = value.gameCounts;
	});

	async function onSubmitCreateGame(timeControl: TimeControl, color: ColorSelect) {
		showCreateModal = false;

		const [data, err] = await services.postCreateGame(timeControl, color);
		if (data) {
			await goto(`play/${data.gameId}`);
		} else {
			const message = createMessage(err);
			addNotification({ type: 'string', message, isSuccess: false });
		}
	}

	onMount(() => {
		client = getClientSession();
	});
	$inspect(client, 'client session');
</script>

<svelte:head>
	<title>Hexchess</title>
</svelte:head>
<Banner />
<CreateGame title="Create a Game?" show={showCreateModal} onSubmit={onSubmitCreateGame} onClose={() => (showCreateModal = false)} />
<div class="center-horizontal-container">
	<div class="center-vertical-container index-container">
		<div>
			{#if client != null}
				<div class="tabs-group">
					<button class="tab" class:tab-selected={!showSelf} onclick={() => showSelf = false}>
						Public Games
					</button>
					<button class="tab" class:tab-selected={showSelf} onclick={() => showSelf = true}>
						My Games
					</button>
				</div>
			{/if}
			<table class="table-container chess-table">
				<thead>
				<tr>
					<th style="width: 37%">White</th>
					<th style="width: 37%">Black</th>
					<th style="width: 26%">Time</th>
				</tr>
				</thead>
				<tbody>
				{#each chessList as chess (chess.id)}
					<tr class="row-hover chess-table-row" style="height: {chessRowHeight}px" onclick={() => goto(`/play/${chess.id}`)}>
						<td>
							{#if chess.whitePlayer}
								{chess.whitePlayer.name}
								<img class="flag" src="/flags/{chess.whitePlayer.country}.png" alt="" />
							{:else}
							<span>
								-
							</span>
							{/if}
						</td>
						<td>
							{#if chess.blackPlayer}
								{chess.blackPlayer.name}
								<img class="flag" src="/flags/{chess.blackPlayer.country}.png" alt="" />
							{:else}
							<span>
								-
							</span>
							{/if}
						</td>
						<td>
							{formatTimeControl(chess.timeControl)}
						</td>
					</tr>
				{/each}
				{#if bottomPadding > 0}
					<tr style="height: {bottomPadding}px">
						<td></td>
						<td></td>
						<td></td>
					</tr>
				{/if}
				</tbody>
			</table>
		</div>
		<div class="buttons-wrapper">
			<button class="button button-grey" id="challenge-button" onclick={() => (showCreateModal = true)}>
				Play a Friend
			</button>
			<button class="button button-grey" id="challenge-button">
				Find a Match
			</button>
			<a href="/sandbox">
				<button class="button button-grey" id="challenge-button">
					Try Sandbox
				</button>
			</a>
			<div class="counts-wrapper">
				<b> {userCounts} </b> players
			</div>
			<div>
				<b> {gameCounts} </b> games in play
			</div>
		</div>
	</div>
</div>

<style>
	.counts-wrapper {
		margin-top: 30px;
	}

	.buttons-wrapper {
		display: flex;
		flex-direction: column;
		gap: 15px;
	}

	.chess-table-row {
		border: 1px solid rgb(60, 60, 60);
	}

	.chess-table {
		width: 700px;
	}

	.index-container {
		gap: 30px;
	}
</style>
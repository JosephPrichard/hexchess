<script lang="ts">
	import CreateGame from '$lib/components/modals/CreateGame.svelte';
	import { makeMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import { goto } from '$app/navigation';
	import { type ChessModel, type ColorSelect, type SessionModel, type GameMode, GameModeNameMap } from '$lib/api/models';
	import services, { baseURL } from '$lib/api/services';
	import { onMount } from 'svelte';
	import { getClientSession } from '$lib/utils/storage';
	import { chessRowHeight, maxChessRows } from './globals';
	import Banner from '$lib/Banner.svelte';
	import { wasm } from '$lib/api/wasm';

	export interface IndexProps {
		chessList: ChessModel[];
		selfChessList: ChessModel[];
		showCreateModal?: boolean;
		fen?: string;
	}

	const { data: props }: { data: IndexProps } = $props();

	const { addNotification } = getNotificationsContext();

	let countSse: EventSource | undefined = undefined;

	let showSelf = $state(false);
	let showCreateModal = $state(props.showCreateModal || false);
	let fen = $state(props.fen || '');
	let userCounts = $state(0);
	let gameCounts = $state(0);
	let client: SessionModel | null = $state(null);

	function connectCountEvents() {
		countSse = new EventSource(`${baseURL()}/events/count`);
		countSse.addEventListener('meta', (event) => {
			console.log('SSE: COUNT_EVENTS meta', event.data);
		});
		countSse.addEventListener('activeCountEvents', (event) => {
			console.log('SSE: COUNT_EVENTS activeCountEvents', event.data);
			const count = JSON.parse(event.data).count;
			if (!Number.isNaN(count)) {
				userCounts = count;
			} else {
				console.error('Invalid count received from server: ', count);
			}
		});
		countSse.addEventListener('gameCountEvents', (event) => {
			console.log('SSE: COUNT_EVENTS gameCountEvents', event.data);
			const count = JSON.parse(event.data).count;
			if (!Number.isNaN(count)) {
				gameCounts = count;
			}  else {
				console.error('Invalid count received from server: ', count);
			}
		});
	}

	onMount(() => {
		connectCountEvents();
		return () => {
			if (countSse) countSse.close();
		};
	});

	onMount(async () => {
		client = getClientSession();
		fen = await wasm.boardToFen((await wasm.getInitialGame()).board);
	});
	$inspect(client, 'client session');

	async function onSubmitCreateGame(mode: GameMode, color: ColorSelect) {
		const [data, err] = await services.postCreateGame(mode, color, fen);
		if (data) {
			await goto(`play/${data.gameId}`);
		} else {
			addNotification({ type: 'string', message: makeMessage(err), isSuccess: false });
		}
	}

	const chessList = $derived.by(() => showSelf ? props.selfChessList : props.chessList);
	const bottomPadding = $derived.by(() => (chessRowHeight * maxChessRows) - (chessRowHeight * chessList.length));
</script>

<svelte:head>
	<title>Hexchess</title>
</svelte:head>
<Banner />
<CreateGame title="Create a Game?" bind:show={showCreateModal} onSubmit={onSubmitCreateGame} bind:fen={fen}/>
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
							{#if chess.whitePlayer && chess.whitePlayer.present}
								{chess.whitePlayer.name}
								<img class="flag" src="/flags/{chess.whitePlayer.country}.png" alt="" />
							{:else}
							<span>
								-
							</span>
							{/if}
						</td>
						<td>
							{#if chess.blackPlayer && chess.blackPlayer.present}
								{chess.blackPlayer.name}
								<img class="flag" src="/flags/{chess.blackPlayer.country}.png" alt="" />
							{:else}
							<span>
								-
							</span>
							{/if}
						</td>
						<td>
							{GameModeNameMap[chess.mode] || "Unknown"}
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
<script lang="ts">
	import type { GameOutputMsg, GameState, PlayerView } from '$lib/models';
	import Banner from '$lib/components/Banner.svelte';
	import { baseURL, postTempSession, unwrap } from '$lib/api';
	import { createMessage } from '$lib/error';
	import { getNotificationsContext } from '$lib/context';
	import MoveListView from '$lib/components/chess/MoveList.svelte';
	import ChessBoardView from '$lib/components/chess/Board.svelte';

	export interface PlayProps {
		gameId: string
	}

	const { data: props }: { data: PlayProps } = $props();

	const { addNotification } = getNotificationsContext();

	let gameState: GameState | undefined = $state(undefined);
	let selfPlayer: PlayerView | undefined = $state(undefined);
	let messages: string[] = $state([]);

	let ws: WebSocket | undefined = undefined;
	let connectTries = 0;

	function onMessage(data: GameOutputMsg) {
		console.log('Received message', data);
		switch (data.type) {
		case 'JOIN':
			gameState = data.gameState;
			break;
		case 'CONNECT':
			selfPlayer = data.player;
			break;
		case 'MOVE':
			if (gameState) {
				gameState.game = data.game;
				gameState.moveList.push(data.move);
			}
			break;
		case 'FORFEIT':
			gameState = data.gameState;
			break;
		case 'CHAT':
			messages.push(data.message)
			break;
		case 'ERROR':
			const message = createMessage(data.message);
			addNotification({ type: 'string', message, isSuccess: true, duration: 3000 });
			break;
		}
	}

	function connectGame(gameId: string) {
		const timeout = connectTries !== 0 ? Math.pow(2, connectTries) * 1000 : 0;
		console.log(`Trying to connect to game=${gameId} in timeout=${timeout}`);
		setTimeout(
			async () => {
				let { ok, resp: sessionId, err } = await unwrap(postTempSession());
				console.log(`Retrieved temporary sessionId=${sessionId}`);
				if (ok && sessionId) {
					const params = new URLSearchParams({ sessionId });
					let url = `${baseURL}/connections/games/${gameId}?${params}`;

					ws = new WebSocket(url);
					ws.addEventListener('open', () => {
						console.log(`Connected to game=${gameId} successfully!`);
						connectTries = 0;
					});
					ws.addEventListener('message', (event) => {
						const data: GameOutputMsg = JSON.parse(event.data);
						onMessage(data);
					});
					ws.addEventListener('error', () => {
						console.log(`Disconnected from game=${gameId} with error, trying to reconnect with ${connectTries} tries`);
						connectTries += 1;
						connectGame(gameId);
					});
				} else {
					const message = createMessage(err);
					addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
				}
			},
			timeout
		);
	}

	$effect(() => {
		connectGame(props.gameId);
		return () => {
			if (ws) {
				ws.close();
			}
		};
	});

	$inspect(gameState, selfPlayer);
</script>

<svelte:head>
	<title>Play - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		{#if gameState}
			{@const game = gameState.game}
			{@const whitePlayer = gameState.whitePlayer}
			{@const blackPlayer = gameState.blackPlayer}
			{@const isPlayingAsBlack = selfPlayer?.id === blackPlayer?.id}
			<ChessBoardView board={game.board} isBlackPerspective={isPlayingAsBlack} />
			<div class="move-table">
				<div class="move-table-header">
					{#if whitePlayer}
						<div class="move-table-header-elem">
							<a href="/players/{whitePlayer.id}" class="text-ul">
								<b>{whitePlayer.name}</b>
							</a>
							<img class="flag" src="/flags/{whitePlayer.country}.png" alt="" />
							{#if whitePlayer.elo}
								<span>({whitePlayer.elo})</span>
							{/if}
						</div>
					{/if}
					{#if blackPlayer}
						<div class="move-table-header-elem">
							<a href="/players/{blackPlayer.id}" class="text-ul">
								<b>{blackPlayer.name}</b>
							</a>
							<img class="flag" src="/flags/{blackPlayer.country}.png" alt="" />
							{#if blackPlayer.elo}
								<span>({blackPlayer.elo})</span>
							{/if}
						</div>
					{/if}
				</div>
				<MoveListView moveList={gameState.moveList} />
				<div class="move-table-footer">
					<div class="message-wrapper">
						{#each messages as message}
							<div class="game-message">
								{message}
							</div>
						{/each}
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

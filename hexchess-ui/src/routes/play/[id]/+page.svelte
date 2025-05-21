<script lang="ts">
	import { type ChatMsg, type GameOutputMsg, type GameState, type PlayerView, whiteTurn } from '$lib/models';
	import Banner from '$lib/components/Banner.svelte';
	import { appBaseURL, baseURL, postTempSession, unwrap } from '$lib/api';
	import { createMessage } from '$lib/error';
	import { getNotificationsContext } from '$lib/context';
	import MoveListView from '$lib/components/chess/MoveList.svelte';
	import ChessBoardView from '$lib/components/chess/Board.svelte';
	import { formatTimeControl } from '$lib/format';
	import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
	import CrossIcon from '$lib/components/icons/CrossIcon.svelte';
	import FlagIcon from '$lib/components/icons/FlagIcon.svelte';

	export interface PlayProps {
		gameId: string
	}

	const { data: props }: { data: PlayProps } = $props();
	const link = $derived(`${appBaseURL}/play/${props.gameId}`);

	const { addNotification } = getNotificationsContext();

	let gameState: GameState | undefined = $state(undefined);
	let selfPlayer: PlayerView | undefined = $state(undefined);
	let chats: ChatMsg[] = $state([]);
	let chat = $state("");
	let completeMessage: string | undefined = $state(undefined);

	const isStarted = $derived(() => gameState?.whitePlayer && gameState?.blackPlayer);

	let ws: WebSocket | undefined = undefined;
	let connectTries = 0;

	function onSubmitChat(e: KeyboardEvent) {
		if (e.key === "Enter" && ws && chat.length > 0) {
			ws.send(JSON.stringify({ type: 'TEXT', message: chat }));
			chat = "";
		}
	}

	async function onClickCopy() {
		await navigator.clipboard.writeText(link);
		addNotification({ type: 'string', message: "Copied to clipboard!", isSuccess: true, duration: 2000 });
	}

	function onClickCross() {

	}

	function onClickFlag() {

	}

	function onMessage(data: GameOutputMsg) {
		console.log('Received message', data);
		switch (data.type) {
			case 'JOIN':
				if (gameState) {
					gameState.whitePlayer = data.whitePlayer;
					gameState.blackPlayer = data.blackPlayer;
				}
				break;
			case 'START':
				selfPlayer = data.selfPlayer;
				gameState = data.gameState;
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
				chats.push(data)
				break;
			case 'ERROR':
				const message = createMessage(data.message);
				addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
				break;
		}
	}

	function connectGame(gameId: string) {
		const timeout = connectTries !== 0 ? Math.pow(2, connectTries) * 1000 : 0;
		console.log(`Trying to connect to game=${gameId} in timeout=${timeout}`);
		setTimeout(
			async () => {
				let { ok, resp: sessionId, err } = await unwrap(postTempSession());
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
</script>

<svelte:head>
	<title>Play - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		{#if gameState && isStarted()}
			{@const game = gameState.game}
			{@const isPlayingAsWhite = selfPlayer?.id === gameState.whitePlayer?.id}
			{@const isTurn = gameState.game.board.turn === whiteTurn && isPlayingAsWhite}
			{@const opponentPlayer = isPlayingAsWhite ? gameState.blackPlayer : gameState.whitePlayer}
			<ChessBoardView board={game.board} isWhitePerspective={isPlayingAsWhite} />
			<div class="side-table table-wrapper">
				<div class="side-table-header player-panel">
					{#if opponentPlayer}
						<div class="side-table-header-elem text-xsm">
							<div class="turn-circle" class:turn-circle-green={!isTurn}></div>
							<a href="/players/{opponentPlayer.id}" class="text-ul">
								<b>{opponentPlayer.name}</b>
							</a>
							<img class="flag-md" src="/flags/{opponentPlayer.country}.png" alt="" />
							{#if opponentPlayer.elo}
								<span>({opponentPlayer.elo})</span>
							{/if}
						</div>
					{/if}
				</div>
				<div class="growing-scrollbox">
					<MoveListView moveList={gameState.moveList} />
					{#if completeMessage}
						<div class="completed-message">
							{completeMessage}
						</div>
					{/if}
				</div>
				<div class="icons">
					<button class="button-transparent svg-container" style:padding-top="10px" onclick={onClickCross}>
						<CrossIcon />
					</button>
					<button class="button-transparent svg-container" style:padding-top="10px" onclick={onClickFlag}>
						<FlagIcon />
					</button>
				</div>
				<div class="side-table-footer player-panel">
					{#if selfPlayer}
						<div class="side-table-header-elem text-xsm">
							<div class="turn-circle" class:turn-circle-green={isTurn}></div>
							<a href="/players/{selfPlayer.id}" class="text-ul">
								<b>{selfPlayer.name}</b>
							</a>
							<img class="flag-md" src="/flags/{selfPlayer.country}.png" alt="" />
							{#if selfPlayer.elo}
								<span>({selfPlayer.elo})</span>
							{/if}
						</div>
					{/if}
				</div>
			</div>
		{:else if gameState}
			<div class="panel lobby">
				<div class="text-lg" style:margin-bottom="20px">
					Challenge to a game
				</div>
				<div class="text-sm" style:margin-bottom="30px">
					{formatTimeControl(gameState.timeControl)}
				</div>
				<div style:margin-bottom="10px">
					To invite someone to play, send them this URL.
				</div>
				<div class="text-outline text-xsm" style:margin-bottom="10px">
					<span class="svg-container">
						<span style:margin-right="10px">
							{link}
						</span>
						<button class="svg-container invisible-button copy-button" onclick={onClickCopy}>
							<ClipboardIcon/>
						</button>
					</span>
				</div>
				<div style:margin-bottom="10px">
					The first person who visits the link will be your opponent.
				</div>
			</div>
		{/if}
	</div>
</div>

<style>
    .text-outline {
        width: fit-content;
        border: 1px solid rgb(58, 58, 58);
        border-radius: 5px;
        padding: 5px 15px 5px 15px;
        line-height: 25px;
    }

    .copy-button {
        padding: 2px;
    }

    .copy-button:hover {
        background-color: rgb(58, 58, 58);
        border-radius: 2px;
    }

    .player-panel {
        padding-top: 15px;
        padding-bottom: 15px;
    }

    .lobby {
        width: 600px;
        height: 500px;
    }

    .table-wrapper {
        margin-top: 15%;
        margin-bottom: 15%;
    }

    .icons {
        text-align: center;
        padding-bottom: 5px;
		padding-top: 5px;
        background-color: rgb(44, 44, 44);
        border-top: 1px solid rgb(58, 58, 58);
    }

	.completed-message {
		text-align: center;
		margin-top: 10px;
		margin-bottom: 10px;
        font-style: italic;
	}

	.turn-circle {
		display: inline-block;
        width: 7px;
        height: 7px;
		margin-right: 5px;
        border-radius: 50%;
        border: 2px solid white;
        background-color: transparent;
	}

	.turn-circle-green {
		border-color: #78b13f;
		background-color: #78b13f;
	}
</style>
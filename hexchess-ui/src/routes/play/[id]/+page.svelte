<script lang="ts">
	import { type ChatMsg, type GameOutputMsg, type ChessRoom, type PlayerView, whiteTurn, piecenames } from '$lib/models';
	import Banner from '$lib/components/Banner.svelte';
	import { appBaseURL, baseURL, postTempSession, unwrap } from '$lib/api';
	import { createMessage } from '$lib/error';
	import { getNotificationsContext } from '$lib/context';
	import MoveListView from '$lib/components/chess/MoveList.svelte';
	import ChessBoardView from '$lib/components/chess/Board.svelte';
	import { formatTimeControl, formatTimer } from '$lib/format';
	import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
	import CrossIcon from '$lib/components/icons/CrossIcon.svelte';
	import FlagIcon from '$lib/components/icons/FlagIcon.svelte';
	import SettingsIcon from '$lib/components/icons/SettingsIcon.svelte';
	import UndoIcon from '$lib/components/icons/UndoIcon.svelte';

	export interface PlayProps {
		gameId: string
	}

	const { data: props }: { data: PlayProps } = $props();
	const link = $derived(`${appBaseURL}/play/${props.gameId}`);

	const { addNotification } = getNotificationsContext();

	let room: ChessRoom | undefined = $state(undefined);
	let selfPlayer: PlayerView | undefined = $state(undefined);
	let chats: ChatMsg[] = $state([]);
	let chat = $state("");
	let completeMessage: string | undefined = $state(undefined);
	let whiteTimer: number | undefined = $state(undefined);
	let blackTimer: number | undefined = $state(undefined);

	const isStarted = $derived(() => room?.whitePlayer && room?.blackPlayer);

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

	function onClickForfeit() {

	}

	function onClickUndo() {

	}

	function onClickSettings() {

	}

	function onMessage(data: GameOutputMsg) {
		console.log('Received message', data);
		switch (data.type) {
			case 'JOIN':
				if (room) {
					room.whitePlayer = data.whitePlayer;
					room.blackPlayer = data.blackPlayer;
				}
				break;
			case 'START':
				selfPlayer = data.selfPlayer;
				room = data.room;
				break;
			case 'MOVE':
				if (room) {
					room.game = data.game;
					room.moveList.push(data.move);
				}
				break;
			case 'FORFEIT':
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
		{#if room && isStarted()}
			{@const game = room.game}
			{@const isPlayingAsWhite = selfPlayer?.id === room.whitePlayer?.id}
			{@const isTurn = room.game.board.turn === whiteTurn && isPlayingAsWhite}
			{@const opponentPlayer = isPlayingAsWhite ? room.blackPlayer : room.whitePlayer}
			{@const selfTimer = isPlayingAsWhite ? whiteTimer : blackTimer}
			{@const opponentTimer = isPlayingAsWhite ? blackTimer : whiteTimer}
			{@const selfTakenPieces = isPlayingAsWhite ? game.takenWhitePieces : game.takenBlackPieces}
			{@const opponentTakenPieces = isPlayingAsWhite ? game.takenBlackPieces : game.takenWhitePieces}
			<ChessBoardView board={game.board} isWhitePerspective={isPlayingAsWhite} />
			<div class="side-table-wrapper">
				<div class="taken-pieces">
					{#each opponentTakenPieces as piece}
						<div class="piece-icon-wrapper">
							<img class="piece-icon" src="/pieces/{piecenames[piece]}.png" draggable={false} alt="" />
						</div>
					{/each}
				</div>
				{#if opponentTimer}
					<div class="timer" class:timer-warn={opponentTimer < 15000}>
						{formatTimer(opponentTimer)}
					</div>
				{/if}
				<div class="side-table move-table-wrapper">
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
						<MoveListView moveList={room.moveList} />
						{#if completeMessage}
							<div class="completed-message">
								{completeMessage}
							</div>
						{/if}
					</div>
					<div class="icons">
						<button title="Forfeit" class="button-transparent svg-container" style:padding-top="10px" onclick={onClickForfeit}>
							<FlagIcon />
						</button>
						<button title="Undo Move" class="button-transparent svg-container" style:padding-top="10px" onclick={onClickUndo}>
							<UndoIcon />
						</button>
						<button title="Settings" class="button-transparent svg-container" style:padding-top="10px" onclick={onClickSettings}>
							<SettingsIcon />
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
				{#if selfTimer}
					<div class="timer" class:timer-warn={selfTimer < 15000}>
						{formatTimer(selfTimer)}
					</div>
				{/if}
				<div class="taken-pieces">
					{#each selfTakenPieces as piece}
						<div class="piece-icon-wrapper">
							<img class="piece-icon" src="/pieces/{piecenames[piece]}.png" draggable={false} alt="" />
						</div>
					{/each}
				</div>
			</div>
		{:else if room}
			<div class="panel lobby">
				<div class="text-lg" style:margin-bottom="20px">
					Challenge to a game
				</div>
				<div class="text-sm" style:margin-bottom="30px">
					{formatTimeControl(room.timeControl)}
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
        border: 2px solid #B4B4B4;
        background-color: transparent;
	}

	.turn-circle-green {
		border-color: #78b13f;
		background-color: #78b13f;
	}

	.timer {
		border-radius: 6px;
		font-weight: bold;
        background-color: rgba(42, 42, 42);
		color: rgb(160, 160, 160);
		font-size: 55px;
		font-family: 'digital-clock-font';
        letter-spacing: 0.2rem;
		text-align: center;
		padding: 20px;
	}

	.timer-warn {
		color: rgba(255, 10, 10, 0.6);
	}

	.side-table-wrapper {
        display: flex;
        flex-direction: column;
		margin-top: auto;
		margin-bottom: auto;
	}

    .move-table-wrapper {
        margin-top: 10px;
        margin-bottom: 10px;
		height: 350px;
    }

	.taken-pieces {
        width: 280px;
        display: flex;
        justify-content: center;
        flex-wrap: wrap;
    }

    .piece-icon {
        width: 35px;
		height: 35px;
		margin-right: -20px;
    }

	.piece-icon-wrapper {
		display: inline-block;
	}
</style>
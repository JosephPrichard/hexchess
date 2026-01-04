<script lang="ts">
	import services, { appBaseURL, baseURL } from '$lib/api/services';
	import { codes, makeMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
	import FlagIcon from '$lib/components/icons/FlagIcon.svelte';
	import SettingsIcon from '$lib/components/icons/SettingsIcon.svelte';
	import UndoIcon from '$lib/components/icons/UndoIcon.svelte';
	import PieceList from '$lib/components/chess/PieceList.svelte';
	import PlayerPanel from '$lib/components/user/PlayerPanel.svelte';
	import { type ChatMsg, type ChatOutput, type ChessGame, GameInput, GameOutput, type PlayerState } from '$lib/pb/messages';
	import type { Hex } from '$lib/api/models';
	import { deserializeHexList } from '$lib/utils/chess.js';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import { getInitialGameWasm, getMoveNotationsWasm } from '$lib/api/wasm';
	import { formatTimer } from '$lib/utils/format';
	import { onMount } from 'svelte';
	import Banner from '$lib/Banner.svelte';
	import Error from '$lib/Error.svelte';

	const maxTimeout = 2500;
	const successConnThresholdTime = 5000;

	export interface PlayProps {
		gameId: string
		gameExists?: boolean
	}

	const { data: props }: { data: PlayProps } = $props();
	const link = $derived(`${appBaseURL()}/play/${props.gameId}`);

	const { addNotification } = getNotificationsContext();

	let game = $state<ChessGame | undefined>(undefined);
	let whitePlayer = $state<PlayerState | undefined>(undefined);
	let blackPlayer = $state<PlayerState | undefined>(undefined);

	let selfPlayer: PlayerState | undefined = $state(undefined);

	let whiteTimer: number | undefined = $state(undefined);
	let blackTimer: number | undefined = $state(undefined);

	let chats: (ChatOutput | ChatMsg)[] = $state([]);
	let chatText = $state("");

	let selection = makeSelectionState();

	interface ConnectionState {
		tries: number
		ws?: WebSocket
		setAt?: Date
	}
	let connState = $state<ConnectionState>({ tries: 0 });

	async function onClickCopy() {
		await navigator.clipboard.writeText(link);
		addNotification({ type: 'string', message: "Copied share link", isSuccess: true, duration: 2000 });
	}

	function onClickForfeit() {}

	function onClickUndo() {}

	function onClickSettings() {}

	function onInputChat(e: KeyboardEvent) {
		if (e.key === 'Enter' && chatText.length > 0) {
			const output: GameInput = {
				value: {
					oneofKind: 'chat',
					chat: { message: chatText }
				}
			};
			connState.ws?.send?.(GameInput.toBinary(output));
		}
	}

	function onSelectPiece(hex: Hex) {
		selection.select(game, hex);
	}

	function handleMessage(data: GameOutput) {
		const kind = data.value.oneofKind;
		if (kind === 'init') {
			const init = data.value.init;
			game = init?.state?.game;
			whitePlayer = init?.state?.whitePlayer;
			blackPlayer = init?.state?.blackPlayer;
			selfPlayer = init.self;
		} else if (kind === 'bgInit') {
			const init = data.value.bgInit;
			chats = init?.chats;
		} else if (kind === 'players') {
			const players = data.value.players;
			whitePlayer = players.whitePlayer;
			blackPlayer = players.blackPlayer;
		} else if (kind === 'move') {
			const move = data.value.move;
			game = move.game;
		} else if (kind === 'forfeit') {

		} else if (kind === 'chat') {
			chats.push(data.value.chat);
		} else if (kind === 'error') {
			const message = makeMessage(data.value.error.message);
			addNotification({ type: 'string', message, isSuccess: false });
		}
	}

	async function tryConnect(gameId: string) {
		const [data, err] = await services.postTempSession();
		if (err) {
			console.error(`Failed to create temporary session: ${err.message}`);
			connState.tries += 1;
			return;
		}
		const sessionId = data?.sessionId || "";

		const params = new URLSearchParams({ sessionId, gameId });
		const url = `${baseURL()}/ws/game?${params}`;

		const tempWs = new WebSocket(url);
		tempWs.binaryType = "arraybuffer";
		tempWs.addEventListener('open', () => {
			console.log(`Connected to game=${gameId} sessionId=${sessionId} successfully!`);
			const lastTime = connState.setAt?.getTime() || 0;
			let tries = 0;
			if (new Date().getTime() - lastTime > successConnThresholdTime) {
				tries = 0;
			} else {
				tries = connState.tries + 1;
			}
			connState = { ...connState, tries: tries, ws: tempWs, setAt: new Date() };
		});
		tempWs.addEventListener('message', (event) => {
			if (event.data instanceof ArrayBuffer) {
				const data = GameOutput.fromBinary(new Uint8Array(event.data));
				console.log(`Received ${data.value.oneofKind} message`, data);
				handleMessage(data);
			}
		});
		tempWs.addEventListener('close', () => {
			console.log(`Disconnected from game=${gameId}`);
			connState = { ...connState, tries: connState.tries + 1, ws: undefined };
		});
	}

	$effect(() => {
		const state = connState;
		if (!props.gameExists) return;
		if (!state.ws) {
			const gameId = props.gameId;
			let timeout = state.tries !== 0 ? state.tries * 250 : 0;
			if (timeout > maxTimeout) {
				timeout = maxTimeout;
			}
			console.log(`Trying to connect to game=${gameId} in timeout=${timeout} with tries=${state.tries}`);
			if (timeout > 0) {
				setTimeout(() => tryConnect(gameId), timeout);
			} else {
				tryConnect(gameId);
			}
		}
		return () => {
			if (state.ws) {
				state.ws.close();
				state.ws = undefined;
			}
		};
	});

	onMount(() => {
		getInitialGameWasm().then(initialGame => game = initialGame);
	});

	const isErrorPage = $derived.by(() => connState.tries > 0);

	const awaitingNotList = $derived.by(async () => await getMoveNotationsWasm(game?.moves));

	const isWhitePerspective = $derived.by(() => selfPlayer === undefined || selfPlayer?.id !== blackPlayer?.id);
	const bottomPlayer = $derived(isWhitePerspective ? blackPlayer : whitePlayer);
	const topPlayer = $derived(isWhitePerspective ? whitePlayer : blackPlayer);
	const isBottomTurn = $derived((bottomPlayer !== undefined && game?.board?.isWhiteTurn && bottomPlayer == whitePlayer) || false);
	const isTopTurn = $derived((topPlayer !== undefined && game?.board?.isWhiteTurn && topPlayer == whitePlayer) || false);
	const bottomTimer = $derived(isWhitePerspective ? whiteTimer : blackTimer);
	const topTimer = $derived(isWhitePerspective ? blackTimer : whiteTimer);
	const topTakenPieces = $derived(isWhitePerspective ? game?.takenWhitePieces : game?.takenBlackPieces);
	const bottomTakenPieces = $derived(isWhitePerspective ? game?.takenBlackPieces : game?.takenWhitePieces);
</script>

<svelte:head>
	<title>Play - Hexchess</title>
</svelte:head>
<Banner />
<div class="disconnect-message" style:display={isErrorPage ? '' : 'none'}>
	Disconnected. Attempting to regain a connection...
</div>
{#if !props.gameExists}
	<Error status={404} message="Game not found!"/>
{:else}
	<div class="center-horizontal-container">
		<div class="center-vertical-container" style="align-items: stretch;">
			<div class="panel chat-wrapper">
				Chat room
				<div class="chats">
					{#each chats as chat}
						<div class="chat">
							<b>{chat.player?.name || "-"}</b> : {chat.message}
						</div>
					{/each}
				</div>
				<input class="chat-input" bind:value={chatText} onkeydown={onInputChat}/>
			</div>
			{#if game?.board}
				<Board
					board={game?.board}
					fen={true}
					isWhitePerspective={isWhitePerspective}
					potentialMoves={deserializeHexList(selection.value.potentialMoves?.moves)}
					onSelectPiece={onSelectPiece}
					selected={selection.value.hex}
				/>
			{/if}
			<div class="side-table-wrapper">
				<PieceList pieces={bottomTakenPieces || []} />
				{#if topTimer}
					<div class="timer" class:timer-warn={topTimer < 15000}>
						{formatTimer(topTimer)}
					</div>
				{/if}
				<div class="side-table move-table-wrapper">
					<div class="side-table-header player-panel">
						<PlayerPanel player={bottomPlayer} self={selfPlayer} isTurn={isBottomTurn} />
					</div>
					{#if whitePlayer === undefined || blackPlayer === undefined}
						<div class="growing-scrollbox parent-lobby">
							<div class="lobby-container">
								<div class="spinner"></div>
								<span class="lobby-text">Waiting for opponents...</span>
							</div>
						</div>
					{:else}
						{#await awaitingNotList then notList}
							<MoveList moveList={notList} />
						{/await}
					{/if}
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
						<button title="Settings" class="button-transparent svg-container" style:padding-top="10px" onclick={onClickCopy}>
							<ClipboardIcon />
						</button>
					</div>
					<div class="side-table-header-bottom player-panel">
						<PlayerPanel player={topPlayer} self={selfPlayer} isTurn={isTopTurn} />
					</div>
				</div>
				{#if bottomTimer}
					<div class="timer" class:timer-warn={bottomTimer < 15000}>
						{formatTimer(bottomTimer)}
					</div>
				{/if}
				<PieceList pieces={topTakenPieces || []} />
			</div>
		</div>
	</div>
{/if}

<style>
	.chat-wrapper {
        width: 200px;
		margin-right: 35px;
        padding: 15px;
        display: flex;
        flex-direction: column;
        margin-top: auto;
        margin-bottom: auto;
    }

	.chat {
        white-space: normal;
        word-wrap: break-word;
        overflow-wrap: break-word;
		border-left: 2px solid dodgerblue;
        padding: 2px 2px 2px 10px;
    }

    .chats {
        margin-top: 10px;
        margin-bottom: 10px;
		height: 250px;
        overflow-y: auto;
    }

	.chat-input {
		padding-top: 2px;
        padding-bottom: 2px;
		height: 30px;
	}

    .disconnect-message {
		padding: 20px;
		border-top-left-radius: 4px;
        position: fixed;
        right: 0;
        bottom: 0;
        z-index: 1000;
		color: white;
		background-color: #B7374E;
    }

    .player-panel {
        padding-top: 15px;
        padding-bottom: 15px;
    }

    .icons {
        text-align: center;
        padding-bottom: 5px;
		padding-top: 5px;
        background-color: rgb(44, 44, 44);
        border-top: 1px solid rgb(58, 58, 58);
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
		height: 400px;
    }

    .lobby-container {
        display: flex;
        align-items: center; /* vertically centers spinner with text */
        gap: 8px; /* space between spinner and text */
    }

    .spinner {
        width: 20px;
        height: 20px;
        border: 4px solid rgb(120, 120, 120);; /* blue ring */
        border-top: 4px solid transparent; /* top is transparent for spinning effect */
        border-radius: 50%;
        animation: spin 1s linear infinite;
    }

    @keyframes spin {
        from { transform: rotate(0deg); }
        to   { transform: rotate(360deg); }
    }

    .lobby-text {
        font-family: sans-serif;
        font-size: 16px;
        font-weight: 500;
		color: rgb(120, 120, 120);
    }

    .parent-lobby {
        display: flex;
        justify-content: center;
        align-items: center;
        height: 100vh;
    }
</style>
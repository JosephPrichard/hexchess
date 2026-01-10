<script lang="ts">
	import services, { appBaseURL, baseURL } from '$lib/api/services';
	import { codes, makeMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
	import FlagIcon from '$lib/components/icons/FlagIcon.svelte';
	import UndoIcon from '$lib/components/icons/UndoIcon.svelte';
	import PieceList from '$lib/components/chess/PieceList.svelte';
	import PlayerPanel from '$lib/components/user/PlayerPanel.svelte';
	import { type ChatMsg, type ChatOutput, type ChessGame, type FinishState, GameInput, GameOutput, PingInput, type PlayerState } from '$lib/pb/messages';
	import type { Hex } from '$lib/api/models';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import { getInitialGameWasm, getMoveNotationsWasm } from '$lib/api/wasm';
	import { formatTimer } from '$lib/utils/format';
	import { onMount, tick } from 'svelte';
	import Banner from '$lib/Banner.svelte';
	import Error from '$lib/Error.svelte';
	import ChatIcon from '$lib/components/icons/ChatIcon.svelte';
	import FinishPanel from '$lib/components/user/FinishPanel.svelte';
	import type { Promotion } from '$lib/state/game.svelte';
	import { defaultBoard, defaultGame, makeGame } from '$lib/utils/chess';

	const forfeitModalIds = ["forfeit-modal", "forfeit-button"];
	const maxTimeout = 2500;
	const successConnThresholdTime = 5000;
	const dangerTimerThreshold = 15000;
	const keepAliveTimeOut = 15000;

	export interface PlayProps {
		gameId: string
		gameExists?: boolean
	}

	const { data: props }: { data: PlayProps } = $props();
	const link = $derived(`${appBaseURL()}/play/${props.gameId}`);

	const { addNotification } = getNotificationsContext();

	// game life cycle states taken from ws responses
	let game = $state<ChessGame | undefined>(defaultGame);
	let whitePlayer = $state<PlayerState | undefined>(undefined);
	let blackPlayer = $state<PlayerState | undefined>(undefined);
	let selfPlayer: PlayerState | undefined = $state(undefined);
	let finishState = $state<FinishState | undefined>(undefined);
	let chats: (ChatOutput | ChatMsg)[] = $state([]);
	let gameExpired = $state(false);

	// game life cycle states that are calculated in sync with the server
	let whiteTimer: number | undefined = $state(undefined);
	let blackTimer: number | undefined = $state(undefined);

	// client side states used to interface with the game
	let selection = makeSelectionState();
	let promotion: Promotion | undefined = $state(undefined);
	let chatText = $state("");
	let showSideTable: "CHAT" | "MOVES" = $state("MOVES");
	let showForfeitModal: boolean = $state(false);

	// non-reactive states for background tasks
	let keepAliveInterval: ReturnType<typeof setInterval> | undefined = undefined;

	// network state
	interface ConnectionState {
		tries: number
		ws?: WebSocket
		setAt?: Date
	}
	let connState = $state<ConnectionState>({ tries: 0 });

	// calculated from the server's game and kept in sync
	const isErrorPage = $derived.by(() => connState.tries > 0);
	const awaitingNotList = $derived.by(async () => await getMoveNotationsWasm(game?.moves));
	const currPlayer = $derived.by(() => game?.board?.isWhiteTurn ? whitePlayer : blackPlayer);
	const isWhitePerspective = $derived.by(() => selfPlayer === undefined || selfPlayer.id !== blackPlayer?.id);
	const bottomPlayer = $derived(isWhitePerspective ? blackPlayer : whitePlayer);
	const topPlayer = $derived(isWhitePerspective ? whitePlayer : blackPlayer);
	const isBottomTurn = $derived((bottomPlayer !== undefined && currPlayer?.id === bottomPlayer.id) || false);
	const isTopTurn = $derived((topPlayer !== undefined && currPlayer?.id === topPlayer.id) || false);
	const bottomTimer = $derived(isWhitePerspective ? whiteTimer : blackTimer);
	const topTimer = $derived(isWhitePerspective ? blackTimer : whiteTimer);
	const topTakenPieces = $derived(isWhitePerspective ? game?.takenWhitePieces : game?.takenBlackPieces);
	const bottomTakenPieces = $derived(isWhitePerspective ? game?.takenBlackPieces : game?.takenWhitePieces);

	function onClickToggleChat() {
		switch (showSideTable) {
		case "CHAT":
			showSideTable = "MOVES";
			break;
		case "MOVES":
			showSideTable = "CHAT";
			break;
		}
	}

	async function onClickCopy() {
		await navigator.clipboard.writeText(link);
		addNotification({ type: 'string', message: "Copied share link", isSuccess: true, duration: 2000 });
	}

	function onToggleForfeitModal() {
		showForfeitModal = !showForfeitModal;
	}

	function onCloseForfeitModal() {
		showForfeitModal = false
	}

	function onConfirmForfeit() {
		connState.ws?.send?.(GameInput.toBinary({
			value: {
				oneofKind: 'forfeit',
				forfeit: {}
			}
		}));
	}

	function onClickUndo() {}

	function onInputChat(e: KeyboardEvent) {
		if (e.key !== 'Enter' || chatText.length <= 0) {
			return;
		}
		connState.ws?.send?.(GameInput.toBinary({
			value: {
				oneofKind: 'chat',
				chat: { message: chatText }
			}
		}));
	}

	function onSelectPiece(hex: Hex) {
		selection.select(game, hex);
	}

	function onDeSelectPiece() {
		selection.deSelect();
	}

	function onPieceMove(from: Hex, to: Hex) {
		if (selfPlayer?.id !== currPlayer?.id) {
			return;
		}
		connState.ws?.send?.(GameInput.toBinary({
			value: {
				oneofKind: 'move',
				move: {move: {
					promotion: 0,
					fromFile: from.file,
					fromRank: from.rank,
					toFile: to.file,
					toRank: to.rank,
				}}
			}
		}));
	}

	function onCompletePromotion() {}

	 function handleMessage(data: GameOutput) {
		const kind = data.value.oneofKind;
		if (kind === 'init') {
			const init = data.value.init;
			game = init?.state?.game;
			whitePlayer = init?.state?.whitePlayer;
			blackPlayer = init?.state?.blackPlayer;
			finishState = init?.state?.finishState;
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
			const forfeit = data.value.forfeit;
			finishState = forfeit?.finishState;
		} else if (kind === 'chat') {
			chats.push(data.value.chat);
		} else if (kind === 'error') {
			const code = data.value.error.message;
			switch (code) {
			case codes.errorInvalidMove:
				// this error can occur whenever the user makes a bad move, the UI will just snap the piece back in place instead of sending an error
				break;
			case codes.errorInvalidGame:
				gameExpired = true;
				break;
			default:
				const message = makeMessage(data.value.error.message);
				addNotification({ type: 'string', message, isSuccess: false });
			}
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
		if (gameExpired || !props.gameExists) return;
		if (!state.ws) {
			const gameId = props.gameId;
			let timeout = state.tries !== 0 ? state.tries * 250 : 0;
			if (timeout > maxTimeout) {
				timeout = maxTimeout;
			}
			console.log(`Trying to connect to game=${gameId} in timeout=${timeout} with tries=${state.tries}`);
			if (timeout > 0) {
				setTimeout(() => tryConnect(gameId), 0);
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
		function keepAlive() {
			connState.ws?.send?.(GameInput.toBinary({
				value: {
					oneofKind: 'ping',
					ping: {},
				}
			}));
		}
		function outsideClick(e: MouseEvent) {
			// finds if the click is contained within a child of one of the following IDs
			if (!forfeitModalIds.reduce((acc, id) => acc || (e.target as HTMLElement).closest(`#${id}`) !== null, false)) {
				showForfeitModal = false;
			}
		}
		keepAliveInterval = setInterval(keepAlive, keepAliveTimeOut);
		if (typeof window !== "undefined") {
			window.addEventListener("click", outsideClick);
		}
		return () => {
			if (keepAliveInterval !== undefined) {
				clearInterval(keepAliveInterval);
			}
			if (typeof window !== "undefined") {
				window.removeEventListener("click", outsideClick);
			}
		}
	});

	// onMount(() => {
	// 	getInitialGameWasm().then(initialGame => game = initialGame);
	// });
</script>

<svelte:head>
	<title>Play - Hexchess</title>
</svelte:head>
<Banner />
{#if gameExpired}
	<div class="bottom-right-error">
		Game has expired due to inactivity.
	</div>
{:else if isErrorPage && props.gameExists}
	<div class="bottom-right-error">
		Disconnected. Attempting to regain a connection...
	</div>
{/if}
{#if !props.gameExists}
	<Error status={404} message="Game not found!"/>
{:else}
	<div class="center-horizontal-container">
		<div class="center-vertical-container" style="align-items: stretch;">
			{#if game?.board}
				<Board
					board={game?.board}
					fen={true}
					draggable="turn"
					isWhitePerspective={isWhitePerspective}
					potentialMoves={selection.getPotentialMoves()}
					selected={selection.state.hex}
					promotion={promotion}
					onSelectPiece={onSelectPiece}
					onDeSelectPiece={onDeSelectPiece}
					onDropPiece={onPieceMove}
					onCompletePromotion={onCompletePromotion}
				/>
			{/if}
			<div class="side-table-wrapper">
				<PieceList pieces={bottomTakenPieces || []} />
				{#if topTimer}
					<div class="timer" class:timer-warn={topTimer < dangerTimerThreshold}>
						{formatTimer(topTimer)}
					</div>
				{/if}
				<div class="side-table move-table-wrapper">
					<div class="side-table-header player-panel">
						<PlayerPanel player={bottomPlayer} self={selfPlayer} isTurn={isBottomTurn} />
					</div>
					{#if showSideTable === "CHAT"}
						<div class="growing-scrollbox">
							<div class="chats">
								{#each chats as chat}
									{@const isMe = selfPlayer !== undefined && chat?.player?.id === selfPlayer?.id}
									<div class="chat">
										<b class:self-color={isMe}>{chat.player?.name || "-"}</b> : {chat.message}
									</div>
								{/each}
							</div>
						</div>
						<input class="chat-input" bind:value={chatText} onkeydown={onInputChat}/>
					{:else if showSideTable === "MOVES"}
						{#if whitePlayer === undefined || blackPlayer === undefined}
							<div class="growing-scrollbox parent-lobby">
								<div class="lobby-container">
									<div class="spinner"></div>
									<span class="lobby-text">Waiting for opponents to join...</span>
								</div>
							</div>
						{:else}
							{#await awaitingNotList then notList}
								{#if notList.length > 0}
									<MoveList moveList={notList} />
								{:else}
									<div class="growing-scrollbox moves-empty-text">
										{#if selfPlayer?.id === whitePlayer?.id}
											<div>
												<div>You're playing as white</div>
												<div>(It's your turn)</div>
											</div>
										{:else if selfPlayer?.id === blackPlayer?.id}
											You're playing as black
										{/if}
									</div>
								{/if}
							{/await}
							{#if finishState}
								<FinishPanel state={finishState} whitePlayer={whitePlayer} blackPlayer={blackPlayer}/>
							{/if}
						{/if}
					{/if}
					<div class="icons">
						{#if showForfeitModal}
							<div class="forfeit-anchor" id="forfeit-modal">
								<div class="forfeit-modal panel">
									<div style:margin-bottom="10px">
										Are you sure you want to forfeit?
									</div>
									<button class="yes-button" onclick={onConfirmForfeit}>
										Yes
									</button>
									<button class="no-button" onclick={onCloseForfeitModal}>
										No
									</button>
								</div>
							</div>
						{/if}
						<button title={showSideTable ? "Show Moves" : "Show Chat"} class="button-transparent svg-container" style:padding-top="10px" onclick={onClickToggleChat}>
							<ChatIcon />
						</button>
						{#if !finishState}
							<button id="forfeit-button" title="Forfeit" class="button-transparent svg-container" style:padding-top="10px" onclick={onToggleForfeitModal}>
								<FlagIcon />
							</button>
							<button title="Undo Move" class="button-transparent svg-container" style:padding-top="10px" onclick={onClickUndo}>
								<UndoIcon />
							</button>
						{/if}
<!--						<button title="Settings" class="button-transparent svg-container" style:padding-top="10px" onclick={onClickSettings}>-->
<!--							<SettingsIcon />-->
<!--						</button>-->
						<button title="Share" class="button-transparent svg-container" style:padding-top="10px" onclick={onClickCopy}>
							<ClipboardIcon />
						</button>
					</div>
					<div class="side-table-header-bottom player-panel">
						<PlayerPanel player={topPlayer} self={selfPlayer} isTurn={isTopTurn} />
					</div>
				</div>
				{#if bottomTimer}
					<div class="timer" class:timer-warn={bottomTimer < dangerTimerThreshold}>
						{formatTimer(bottomTimer)}
					</div>
				{/if}
				<PieceList pieces={topTakenPieces || []} />
			</div>
		</div>
	</div>
{/if}

<style>
	.moves-empty-text {
        text-align: center;
        display: grid;
        place-items: center;
        color: rgb(140, 140, 140);
	}

	.forfeit-anchor {
		position: relative;
		width: 0;
		height: 0;
	}

	.forfeit-modal {
		position: absolute;
        width: max-content;
		bottom: 1px;
        padding: 20px;
		border-radius: 4px;
		background-color: rgb(44, 44, 44);
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
        overflow-y: auto;
    }

	.chat-input {
		padding-top: 2px;
        padding-bottom: 2px;
		height: 30px;
		border-radius: 0;
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

	.yes-button {
		cursor: pointer;
		border: 0;
		padding: 10px;
		border-radius: 2px;
		background-color: #80CD32;
	}

	.no-button {
        cursor: pointer;
        border: 0;
        padding: 10px;
        border-radius: 2px;
		background-color: rgb(100, 100, 100);
	}
</style>
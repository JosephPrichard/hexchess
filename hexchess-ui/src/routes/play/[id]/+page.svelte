<script lang="ts">
	import services, { appBaseURL, baseURL } from '$lib/api/services';
	import { codes, makeMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
	import FlagIcon from '$lib/components/icons/FlagIcon.svelte';
	import UndoIcon from '$lib/components/icons/UndoIcon.svelte';
	import TakenTakenList from '$lib/components/chess/TakenList.svelte';
	import PlayerPanel from '$lib/components/user/PlayerPanel.svelte';
	import {
		type BgInitOutput,
		type ChatOutput, ChessBoard,
		type ChessGame,
		type EndState, type ErrorOutput,
		type ForfeitOutput,
		GameOutput,
		type InitOutput, type MoveOutput,
		type PlayersOutput,
		type PlayerState, type UndoOutput
	} from '$lib/pb/messages';
	import type { Hex } from '$lib/api/models';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import { wasm } from '$lib/api/wasm';
	import { formatTimer } from '$lib/utils/format';
	import { onMount } from 'svelte';
	import Banner from '$lib/Banner.svelte';
	import Error from '$lib/Error.svelte';
	import ChatIcon from '$lib/components/icons/ChatIcon.svelte';
	import FinishPanel from '$lib/components/user/FinishPanel.svelte';
	import { makeSandboxState, type PromotionMove } from '../../sandbox/state.svelte.js';
	import { defaultBoard, defaultGame, isPromotion, isValidMove } from '../../../lib/service/chess';
	import { type ConnectionState, sendChatInput, sendForfeitInput, sendMoveInput, sendPingInput, sendUndoInput } from './messages';
	import { CancelPromotion, type BadPromotionType, type MoveAction, NoPromotion, type Promotion } from '$lib/components/chess/types';
	import { makeMoveState } from '$lib/state/move.svelte';
	import Timer from '$lib/components/chess/Timer.svelte';

	const forfeitModalIds = ["forfeit-modal", "forfeit-button"];
	const maxTimeout = 2500;
	const successConnThresholdTime = 5000;
	const keepAliveTimeOut = 15000;

	export interface PlayProps {
		gameId: string
		gameExists?: boolean
	}

	const { data: props }: { data: PlayProps } = $props();

	const { addNotification } = getNotificationsContext();

	// game lifecycle states taken from ws responses
	const gameplay = makeSandboxState();
	let whitePlayer = $state<PlayerState | undefined>(undefined);
	let blackPlayer = $state<PlayerState | undefined>(undefined);
	let selfPlayer: PlayerState | undefined = $state(undefined);
	let endState = $state<EndState | undefined>(undefined);
	let undoPlayerId: bigint | undefined = $state(undefined);
	let chats: ChatOutput[] = $state([]);
	let gameExpired = $state(false);

	// game view states that are calculated from lifecycle states and used to display temporary data
	let moveGame = $state<ChessGame | undefined>(undefined);

	// game lifecycle states that are calculated in sync with the server
	let whiteTimer: number | undefined = $state(undefined);
	let blackTimer: number | undefined = $state(undefined);

	// client side states used to interface with the game
	const selection = makeSelectionState();
	let move = makeMoveState();
	let preloadedMove = $state<MoveAction | undefined>(undefined);
	let chatText = $state("");

	let showMovesTable: boolean = $state(true);
	let showForfeitModal: boolean = $state(false);

	// non-reactive states for client side logic
	let keepAliveInterval: ReturnType<typeof setInterval> | undefined = undefined;
	let chatElement: HTMLElement | null = $state(null);
	let initialBoard: ChessBoard | undefined = undefined;

	// network state
	let connState = $state<ConnectionState>({ tries: 0 });

	// calculated from the server's game state and kept in sync
	const game = $derived.by(() => gameplay.state.game || defaultGame);
	const link = $derived(`${appBaseURL()}/play/${props.gameId}`);
	const isErrorPage = $derived.by(() => connState.tries > 0);
	const notList = $derived.by(() => game?.moves.map((h) => h.notation) ?? []);
	const currPlayer = $derived.by(() => game?.board?.isWhiteTurn ? whitePlayer : blackPlayer);
	const isCurrPlayer = $derived.by(() => selfPlayer?.id === currPlayer?.id);
	const isEitherPlayer = $derived.by(() => whitePlayer?.id !== selfPlayer?.id || blackPlayer?.id !== selfPlayer?.id);
	const isSelfWhite = $derived.by(() => selfPlayer?.id === whitePlayer?.id);
	// const otherPlayer = $derived.by(() => !game?.board?.isWhiteTurn ? whitePlayer : blackPlayer);
	// const isStarted = $derived(whitePlayer && blackPlayer && currPlayer !== undefined);
	const isWhitePerspective = $derived.by(() => selfPlayer === undefined || selfPlayer.id !== blackPlayer?.id);
	const bottomPlayer = $derived(isWhitePerspective ? blackPlayer : whitePlayer);
	const topPlayer = $derived(isWhitePerspective ? whitePlayer : blackPlayer);
	const isBottomTurn = $derived((bottomPlayer !== undefined && currPlayer?.id === bottomPlayer.id) || false);
	const isTopTurn = $derived((topPlayer !== undefined && currPlayer?.id === topPlayer.id) || false);
	const bottomTimer = $derived(isWhitePerspective ? whiteTimer : blackTimer);
	const topTimer = $derived(isWhitePerspective ? blackTimer : whiteTimer);
	const topTakenPieces = $derived(isWhitePerspective ? game?.takenWhitePieces : game?.takenBlackPieces);
	const bottomTakenPieces = $derived(isWhitePerspective ? game?.takenBlackPieces : game?.takenWhitePieces);
	const prevMove = $derived.by(() => game ? game.moves[game.moves.length - 1] : undefined);
	const canForfeit = $derived(!endState);
	const canTakeback = $derived(prevMove !== undefined && !endState);
	const selectedMoveIndex = $derived(move.state.moveIndex !== undefined ? move.state.moveIndex : game.moves.length-1)
	// const displayGame = $derived(game);

	// client side callbacks and interactivity. used to control modals/inputs that ultimately send websocket messages to the game server.
	const onToggleForfeitModal = () => showForfeitModal = !showForfeitModal;
	const onCloseForfeitModal = () => showForfeitModal = false;

	function onConfirmForfeit() {
		sendForfeitInput(connState);
		onCloseForfeitModal();
	}

	const onCreateUndo = () => sendUndoInput(connState, "CREATE");
	const onAcceptUndo = () => sendUndoInput(connState, "ACCEPT");
	const onRejectUndo = () => sendUndoInput(connState, "REJECT");

	function onInputChat(e: KeyboardEvent) {
		if (e.key !== 'Enter' || chatText.length <= 0) return;
		sendChatInput(connState, chatText);
		chatText = "";
	}

	const onClickToggleChat = () => showMovesTable = !showMovesTable;

	async function onSelectMove(index: number) {
		move.selectMove(index);
		// if (index === game.moves.length-1) {
		// 	// selecting the last move is a special case - it means we are no longer viewing a specific move state (since the last move state is the current one)
		// 	moveGame = undefined;
		// } else {
		// 	moveGame = await gameAtMoveIndex(initialBoard || defaultBoard, game, index);
		// }
		onDeSelectPiece();
	}

	const onSelectPiece = (hex: Hex) => selection.select(game, hex);
	const onDeSelectPiece = () => selection.deSelect();

	function onPieceMove(from: Hex, to: Hex) {
		if (moveGame) return; // we can't make any moves on a previous game
		if (!isValidMove(game, {from, to}, isSelfWhite)) return;

		const move = {from, to, promotion: NoPromotion};
		if (isPromotion(to)) {
			gameplay.setPromotion({from, to});
		} else if (isCurrPlayer) {
			sendMoveInput(connState, move);
		} else if (isEitherPlayer) {
			preloadedMove = move;
		}
	}

	function onCompletePromotion(promoMove: PromotionMove | undefined, promotion: Promotion | BadPromotionType) {
		if (promotion == CancelPromotion) {
			gameplay.revertPromotion();
		} else {
			// we don't need to validate moves by the time we complete a promotion - BUT we do need to check if this is a preloaded promotion or not
			if (promoMove === undefined) return;
			const move = {...promoMove, promotion };
			if (isCurrPlayer) {
				sendMoveInput(connState, move);
			} else if (isEitherPlayer) {
				preloadedMove = move;
				gameplay.revertPromotion();
			}
		}
		gameplay.setPromotion(undefined);
	}

	function checkPreloadedMove() {
		if (!preloadedMove) return;

		if (preloadedMove.promotion.kind == 0) {
			onPieceMove(preloadedMove.from, preloadedMove.to);
		} else {
			onCompletePromotion(preloadedMove, preloadedMove.promotion);
		}
		preloadedMove = undefined;
	}

	async function onClickCopy() {
		await navigator.clipboard.writeText(link);
		addNotification({ type: 'string', message: "Copied share link", isSuccess: true, duration: 2000 });
	}

	// handles every single message type that can be received from the server and keeps the client side state in sync with the server state.
	function handleMessage(data: GameOutput) {
		const kind = data.value.oneofKind;
		switch (kind) {
		case "init":
			handleInit(data.value.init);
			break;
		case "bgInit":
			handleBgInit(data.value.bgInit);
			break;
		case "players":
			handlePlayers(data.value.players);
			break;
		case "move":
			handleMove(data.value.move);
			break;
		case "forfeit":
			handleForfeit(data.value.forfeit);
			break;
		case "chat":
			handleChat(data.value.chat);
			break;
		case "undo":
			handleUndo(data.value.undo);
			break;
		case "error":
			handleError(data.value.error);
			break;
		}
	}

	function handleInit(init: InitOutput) {
		const state = init.state;
		if (state?.game) gameplay.setGame(state.game);
		whitePlayer = state?.whitePlayer;
		blackPlayer = state?.blackPlayer;
		endState = state?.endState;
		selfPlayer = init?.self;
		undoPlayerId = state?.undoId;
		initialBoard = state?.initialBoard;
	}

	function handleBgInit(init: BgInitOutput) {
		chats = init?.chats;
	}

	function handlePlayers(players: PlayersOutput) {
		whitePlayer = players.whitePlayer;
		blackPlayer = players.blackPlayer;
	}

	function handleMove(move: MoveOutput) {
		gameplay.revertPromotion();
		if (move?.game) gameplay.setGame(move.game);
		checkPreloadedMove();
		undoPlayerId = undefined;
	}

	function handleForfeit(forfeit: ForfeitOutput) {
		endState = forfeit?.endState;
	}

	function handleChat(chat: ChatOutput) {
		chats = [...chats, chat]; // copy so we can react to this state in the $effect
	}

	function handleUndo(undo: UndoOutput) {
		if (undo.kind === "CREATE")
			undoPlayerId = undo.undoId;
		else if (undo.kind === "ACCEPT" || undo.kind === "REJECT")
			undoPlayerId = undefined
		if (undo.game)
			gameplay.setGame(undo.game);
	}

	function handleError(error: ErrorOutput) {
		const code = error.message;
		switch (code) {
		case codes.errorUndoCurrPlayer:
		case codes.errorInvalidMove:
		case codes.errorStartedGame:
			// these errors can occur whenever the user makes a bad move, the UI will just snap the piece back in place instead of showing an error
			break;
		case codes.errorInvalidGame:
			gameExpired = true;
			break;
		default:
			addNotification({ type: "string", message: makeMessage(code), isSuccess: false });
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
			const lastTime = connState.setAt?.getTime() ?? [];
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
		const keepAlive = () => sendPingInput(connState);
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

	$effect(() => {
		const _ = chats; // this is a hack to get svelte to track changes to the chat data and scroll the element whenever there is a change
		if (chatElement !== null) {
			chatElement.scrollTop = chatElement.scrollHeight;
		}
	});
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
					promotion={gameplay.state.promotion}
					prevMove={prevMove}
					nextMove={preloadedMove}
					onSelectPiece={onSelectPiece}
					onDeSelectPiece={onDeSelectPiece}
					onDropPiece={(from, to) => onPieceMove(from, to)}
					onCompletePromotion={(promotion) => onCompletePromotion(gameplay.state.promotion, promotion)}
				/>
			{/if}
			<div class="side-table-wrapper">
				<TakenTakenList myPieces={bottomTakenPieces} theirPieces={topTakenPieces}/>
				<Timer value={topTimer} size="lg"/>
				<div class="side-table move-table-wrapper">
					<div class="side-table-header player-panel">
						<PlayerPanel player={bottomPlayer} self={selfPlayer} isTurn={isBottomTurn} />
					</div>
					{#if !showMovesTable}
						<div class="growing-scrollbox" bind:this={chatElement}>
							<div class="chats">
								{#each chats as chat}
									{@const isMe = selfPlayer !== undefined && chat?.player?.id === selfPlayer?.id}
									<div class="chat">
										[<span class:chat-self={isMe} class:chat-player={!isMe}> {chat.player?.name || "-"}</span>]
										{chat.message}
									</div>
								{/each}
							</div>
						</div>
						<input class="chat-input" bind:value={chatText} onkeydown={onInputChat} placeholder="Type a message here..."/>
					{:else if showMovesTable}
						{#if endState?.value.oneofKind === "abortState"}
							<div class="growing-scrollbox">
							</div>
							<div class="abort-container">
								<div class="error-box abort-box">
									Game is aborted.
								</div>
							</div>
						{:else if whitePlayer === undefined || blackPlayer === undefined}
							<div class="growing-scrollbox parent-lobby">
								<div class="lobby-container">
									<div class="spinner"></div>
									<span class="lobby-text">Waiting for opponents...</span>
								</div>
							</div>
						{:else}
							{#if notList.length > 0}
								<MoveList moveList={notList} onSelectMove={onSelectMove} selectedMoveIndex={selectedMoveIndex}/>
							{:else if endState === undefined}
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
							{#if endState?.value.oneofKind === "finishState"}
								<FinishPanel state={endState?.value.finishState} whitePlayer={whitePlayer} blackPlayer={blackPlayer}/>
							{/if}
							{#if undoPlayerId === selfPlayer?.id}
								<div class="undo-panel">
									<div class="undo-text">
										Takeback sent...
									</div>
									<button class="undo-reject undo-button" onclick={onRejectUndo}>
										x
									</button>
								</div>
							{:else if undoPlayerId && selfPlayer !== undefined}
								<div class="undo-panel">
									<button class="undo-accept undo-button" onclick={onAcceptUndo}>
										✓
									</button>
									<div class="undo-text">
										Your opponent proposes a takeback.
									</div>
									<button class="undo-reject undo-button" onclick={onRejectUndo}>
										x
									</button>
								</div>
							{/if}
						{/if}
					{/if}
					<div class="side-table-header-bottom-shadow">
						<div class="icons">
							{#if showForfeitModal}
								<div class="modal-button-anchor" id="forfeit-modal">
									<div class="modal-button panel">
										<div style:margin-bottom="10px">
											Are you sure you want to {whitePlayer && blackPlayer ? "forfeit" : "abort the game"}?
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
							<div class="button-wrapper">
								<button title={showMovesTable ? "Show Moves" : "Show Chat"} class="button-transparent svg-container" style:padding-top="10px" onclick={onClickToggleChat}>
									<ChatIcon />
								</button>
								{#if canForfeit}
									{#if selfPlayer?.id === whitePlayer?.id || selfPlayer?.id === blackPlayer?.id}
										<button id="forfeit-button" title="Forfeit" class="button-transparent svg-container" style:padding-top="10px" onclick={onToggleForfeitModal}>
											<FlagIcon />
										</button>
									{/if}
								{/if}
								{#if canTakeback}
									<button title="Undo Move" class="button-transparent svg-container" style:padding-top="10px" onclick={onCreateUndo}>
										<UndoIcon />
									</button>
								{/if}
								<button title="Share" class="button-transparent svg-container" style:padding-top="10px" onclick={onClickCopy}>
									<ClipboardIcon />
								</button>
							</div>
						</div>
						<div class="side-table-header-bottom player-panel">
							<PlayerPanel player={topPlayer} self={selfPlayer} isTurn={isTopTurn} />
						</div>
					</div>
				</div>
				<Timer value={bottomTimer} size="lg"/>
				<TakenTakenList myPieces={topTakenPieces} theirPieces={bottomTakenPieces} />
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

	.modal-button-anchor {
		position: relative;
		width: 0;
		height: 0;
	}

	.modal-button {
		position: absolute;
        width: max-content;
		bottom: 1px;
        padding: 20px;
		border-radius: 4px;
		background-color: rgb(44, 44, 44);
	}

	.chat-self {
        color: rgba(240, 230, 140, 0.7);
	}

	.chat-player {
		color: rgb(120, 120, 120);
	}

	.chat {
		max-width: 100%;
		font-size: 14px;
        white-space: normal;
        word-wrap: break-word;
        overflow-wrap: break-word;
        padding: 2px 2px 2px 10px;
    }

    .chats {
        margin-top: 10px;
        margin-bottom: 10px;
        overflow-y: auto;
		width: 325px;
    }

	.chat-input {
		padding-top: 3px;
        padding-bottom: 2px;
		height: 25px;
		border-radius: 0;
		border: none;
		border-top: 1px solid rgb(58, 58, 58);
		background-color: rgb(44, 44, 44);
		color: rgb(160, 160, 160);
		font-size: 13px;
	}

	.chat-input:focus {
        border-top: 1px solid rgb(120, 120, 120);
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

	.abort-container {
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.abort-box {
        width: calc(100% - 60px);
        margin-left: 30px;
        margin-right: 30px;
		text-align: center;
		margin-bottom: 15px;
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

    .undo-panel {
        font-size: 14px;
        margin: 0;
        display: flex;
        flex-direction: row;
        align-items: center;
    }

    .undo-text {
        border-radius: 2px;
        padding-left: 5px;
        padding-right: 5px;
        display: flex;
		align-items: center;
        justify-content: center;
        flex-grow: 1;
        background-color: rgb(44, 44, 44);
		height: 40px;
    }

	.undo-accept {
        background-color: #80CD32;
	}

	.undo-reject {
        background-color: crimson;
	}

    .undo-button {
        cursor: pointer;
        border: 0;
        padding: 10px;
        border-radius: 2px;
		width: 40px;
		height: 40px;
    }

	.button-wrapper {
		display: flex;
        justify-content: center;
        align-items: center;
	}
</style>
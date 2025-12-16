<script lang="ts">
	import Banner from '$lib/Banner.svelte';
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
	import { type ChessGame, GameOutput, type PlayerState } from '$lib/pb/messages';
	import type { Hex } from '$lib/api/models';
	import { deserializeHexList } from '$lib/utils/chess.js';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import { getInitialGameWasm, getMoveNotationsWasm } from '$lib/api/wasm';
	import { goto } from '$app/navigation';
	import { formatTimer } from '$lib/utils/format';
	import { onMount } from 'svelte';

	export interface PlayProps {
		gameId: string
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

	let selection = makeSelectionState();

	let prevUpdatedAt = new Date(0);
	let isForfeit: boolean = false;
	let ws: WebSocket | undefined = undefined;
	let connectTries = 0;

	async function onClickCopy() {
		await navigator.clipboard.writeText(link);
		addNotification({ type: 'string', message: "Copied share link", isSuccess: true, duration: 2000 });
	}

	function onClickForfeit() {}

	function onClickUndo() {}

	function onClickSettings() {}

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
		} else if (kind === 'players') {
			const players = data.value.players;
			whitePlayer = players.whitePlayer;
			blackPlayer = players.blackPlayer;
		} else if (kind === 'move') {
			const move = data.value.move;
			const updatedAt = new Date(move.updatedAt);
			if (updatedAt.getTime() > prevUpdatedAt.getTime()) {
				game = move.game;
				prevUpdatedAt = updatedAt;
			}
		} else if (kind === 'forfeit') {
			isForfeit = true;
		} else if (kind === 'error') {
			const error = data.value.error;
			switch (error.message) {
			case codes.errorInvalidGame:
				goto(`/`);
				break;
			default:
				const message = makeMessage(error.message);
				addNotification({ type: 'string', message, isSuccess: false });
			}
		}
	}

	async function tryConnect(gameId: string) {
		const [data, err] = await services.postTempSession();
		if (data || err?.status === 401) {
			const params = new URLSearchParams({ sessionId: data?.sessionId || "", gameId });
			const url = `${baseURL()}/ws/game?${params}`;

			ws = new WebSocket(url);
			ws.binaryType = "arraybuffer";
			ws.addEventListener('open', () => {
				console.log(`Connected to game=${gameId} sessionId=${data?.sessionId} successfully!`);
				connectTries = 0;
			});
			ws.addEventListener('message', (event) => {
				if (event.data instanceof ArrayBuffer) {
					const data = GameOutput.fromBinary(new Uint8Array(event.data));
					console.log(`Received ${data.value.oneofKind} message`, data);
					handleMessage(data);
				}
			});
			ws.addEventListener('error', () => {
				console.log(`Disconnected from game=${gameId} with error, trying to reconnect with ${connectTries} tries`);
				connectTries += 1;
				connectGame(gameId);
			});
		} else {
			const message = makeMessage(err);
			addNotification({ type: 'string', message, isSuccess: false });
		}
	}
	function connectGame(gameId: string) {
		const timeout = connectTries !== 0 ? Math.pow(2, connectTries) * 1000 : 0;
		console.log(`Trying to connect to game=${gameId} in timeout=${timeout}`);
		setTimeout(() => tryConnect(gameId), timeout);
	}

	$effect(() => {
		connectGame(props.gameId);
		return () => {
			if (ws) ws.close();
		};
	});

	onMount(() => {
		getInitialGameWasm().then(x => game = x);
	});

	const awaitingNotList = $derived.by(async () => await getMoveNotationsWasm(game?.moves));

	const isPlayingAsWhite = $derived.by(() => selfPlayer?.id !== blackPlayer?.id);
	const isTurn = $derived(game?.board?.isWhiteTurn && isPlayingAsWhite);
	const bottomPlayer = $derived(isPlayingAsWhite ? blackPlayer : whitePlayer);
	const topPlayer = $derived(isPlayingAsWhite ? whitePlayer : blackPlayer);
	const bottomTimer = $derived(isPlayingAsWhite ? whiteTimer : blackTimer);
	const topTimer = $derived(isPlayingAsWhite ? blackTimer : whiteTimer);
	const topTakenPieces = $derived(isPlayingAsWhite ? game?.takenWhitePieces : game?.takenBlackPieces);
	const bottomTakenPieces = $derived(isPlayingAsWhite ? game?.takenBlackPieces : game?.takenWhitePieces);
</script>

<svelte:head>
	<title>Play - Hexchess</title>
</svelte:head>
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		{#if game?.board}
			<Board
				board={game?.board}
				fen={true}
				isWhitePerspective={isPlayingAsWhite}
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
					<PlayerPanel player={bottomPlayer} self={selfPlayer} isTurn={!isTurn} />
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
					<PlayerPanel player={topPlayer} self={selfPlayer} isTurn={isTurn || false} />
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

<style>
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
		height: 350px;
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
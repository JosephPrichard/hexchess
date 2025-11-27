<script lang="ts">
	import Banner from '$lib/Banner.svelte';
	import services, { appBaseURL, baseURL } from '$lib/api/services';
	import { codes, makeMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import { formatTimer } from '$lib/utils/format';
	import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
	import FlagIcon from '$lib/components/icons/FlagIcon.svelte';
	import SettingsIcon from '$lib/components/icons/SettingsIcon.svelte';
	import UndoIcon from '$lib/components/icons/UndoIcon.svelte';
	import PieceList from '$lib/components/chess/PieceList.svelte';
	import PlayerPanel from '$lib/components/user/PlayerPanel.svelte';
	import { type ChessGame, GameOutput, type PlayerState } from '$lib/pb/messages';
	import type { Hex } from '$lib/api/model';
	import { mapHexagons } from '$lib/utils/chess.js';
	import { makeMoveSelectionState } from '$lib/state/selection.svelte';
	import { getMoveNotationsWasm } from '$lib/api/wasm';
	import { goto } from '$app/navigation';

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

	let selection = makeMoveSelectionState();

	let prevUpdatedAt = new Date(0);
	let isForfeit: boolean = false;
	let ws: WebSocket | undefined = undefined;
	let connectTries = 0;

	async function onClickCopy() {
		await navigator.clipboard.writeText(link);
		addNotification({ type: 'string', message: "Copied to clipboard!", isSuccess: true, duration: 2000 });
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

	const awaitingNotList = $derived.by(async () => await getMoveNotationsWasm(game?.moves));
</script>

<svelte:head>
	<title>Play - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		{#if whitePlayer && blackPlayer}
			{@const isPlayingAsWhite = selfPlayer?.id === whitePlayer?.id}
			{@const isTurn = game?.board?.isWhiteTurn && isPlayingAsWhite}
			{@const bottomPlayer = isPlayingAsWhite ? blackPlayer : whitePlayer}
			{@const topPlayer = isPlayingAsWhite ? whitePlayer : blackPlayer}
			{@const bottomTimer = isPlayingAsWhite ? whiteTimer : blackTimer}
			{@const topTimer = isPlayingAsWhite ? blackTimer : whiteTimer}
			{@const topTakenPieces = isPlayingAsWhite ? game?.takenWhitePieces : game?.takenBlackPieces}
			{@const bottomTakenPieces = isPlayingAsWhite ? game?.takenBlackPieces : game?.takenWhitePieces}
			{#if game?.board}
				<Board
					board={game?.board}
					isWhitePerspective={isPlayingAsWhite}
					potentialMoves={mapHexagons(selection.value.potentialMoves?.moves)}
					onSelectPiece={onSelectPiece}
					selectedHexagon={selection.value.hex}
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
						<PlayerPanel player={bottomPlayer} isTurn={!isTurn} />
					</div>
					{#await awaitingNotList then notList}
						<MoveList moveList={notList} />
					{/await}
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
					<div class="side-table-header-bottom player-panel">
						<PlayerPanel player={topPlayer} isTurn={isTurn || false} />
					</div>
				</div>
				{#if bottomTimer}
					<div class="timer" class:timer-warn={bottomTimer < 15000}>
						{formatTimer(bottomTimer)}
					</div>
				{/if}
				<PieceList pieces={topTakenPieces || []} />
			</div>
		{:else}
			<div class="panel lobby">
				<div class="text-lg" style:margin-bottom="20px">
					Challenge to a game
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
							<span style:margin-left="auto">
								<ClipboardIcon/>
							</span>
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
    .player-panel {
        padding-top: 15px;
        padding-bottom: 15px;
    }

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
</style>
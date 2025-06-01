<script lang="ts">
	import Banner from '$lib/components/Banner.svelte';
	import { appBaseURL, baseURL, postTempSession, unwrap } from '$lib/api';
	import { createMessage } from '$lib/error';
	import { getNotificationsContext } from '$lib/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import { formatTimeControl, formatTimer, timeControlIntMap } from '$lib/format';
	import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
	import FlagIcon from '$lib/components/icons/FlagIcon.svelte';
	import SettingsIcon from '$lib/components/icons/SettingsIcon.svelte';
	import UndoIcon from '$lib/components/icons/UndoIcon.svelte';
	import PieceList from '$lib/components/chess/PieceList.svelte';
	import PlayerPanel from '$lib/components/user/PlayerPanel.svelte';
	import { type Chat, type ChessRoom, GameOutput, type Hexagon, type PieceMoves, type Player } from '$lib/messages';

	export interface PlayProps {
		gameId: string
	}

	const { data: props }: { data: PlayProps } = $props();
	const link = $derived(`${appBaseURL}/play/${props.gameId}`);

	const { addNotification } = getNotificationsContext();

	let room: ChessRoom | undefined = $state(undefined);
	let selfPlayer: Player | undefined = $state(undefined);
	let chats: Chat[] = $state([]);
	let chat = $state("");
	let completeMessage: string | undefined = $state(undefined);
	let whiteTimer: number | undefined = $state(undefined);
	let blackTimer: number | undefined = $state(undefined);
	let potentialMoves: PieceMoves | undefined = $state(undefined);
	let selectedHexagon: Hexagon | undefined = $state(undefined);

	const isStarted = $derived(() => room?.whitePlayer && room?.blackPlayer);

	let ws: WebSocket | undefined = undefined;
	let connectTries = 0;

	function onSubmitChat(e: KeyboardEvent) {
		if (e.key === "Enter" && ws && chat.length > 0) {
			ws.send(JSON.stringify({ type: 'CHAT', message: chat }));
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

	function onClickPiece(newSelection: Hexagon) {
		if (!room?.game) {
			return;
		}

		if (newSelection.file === newSelection.file && newSelection.rank == newSelection.rank) {
			potentialMoves = undefined;
			selectedHexagon = undefined;
			return;
		}

		let index = room.game.blackMoves.findIndex(move => newSelection.file === move?.hex?.file && newSelection.rank == move?.hex?.rank);
		if (index !== -1) {
			potentialMoves = room.game.blackMoves[index];
			selectedHexagon = newSelection;
			return;
		} else {
			index = room.game.whiteMoves.findIndex(move => newSelection.file === move?.hex?.file && newSelection.rank == move?.hex?.rank);
			if (index !== -1) {
				potentialMoves = room.game.whiteMoves[index];
				selectedHexagon = newSelection;
				return;
			}
		}

		selectedHexagon = undefined;
	}

	function onMessage(data: GameOutput) {
		switch (data.value.oneofKind) {
			case 'join':
				const join = data.value.join;
				if (room) {
					room.whitePlayer = join.whitePlayer;
					room.blackPlayer = join.blackPlayer;
				}
				break;
			case 'start':
				const start =  data.value.start;
				selfPlayer = start.selfPlayer;
				room = start.room;
				break;
			case 'move':
				const move = data.value.move;
				if (move.pieceMove === undefined) {
					throw new Error("Piece move must be specified, got " + JSON.stringify(move));
				}
				if (room) {
					room.game = move.game;
					room.moveList.push(move.pieceMove);
				}
				break;
			case 'forfeit':
				break;
			case 'chat':
				const chat = data.value.chat;
				chats.push(chat);
				break;
			case 'error':
				const error = data.value.error;
				const message = createMessage(error.message);
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
					ws.binaryType = "arraybuffer";
					ws.addEventListener('open', () => {
						console.log(`Connected to game=${gameId} successfully!`);
						connectTries = 0;
					});
					ws.addEventListener('message', (event) => {
						if (event.data instanceof ArrayBuffer) {
							const data = GameOutput.fromBinary(new Uint8Array(event.data));
							console.log(`Received message with length=${event.data.byteLength} data`, data);
							onMessage(data);
						}
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
			{@const board = room.game?.board}
			{@const isPlayingAsWhite = selfPlayer?.id === room.whitePlayer?.id}
			{@const isTurn = board?.isWhiteTurn && isPlayingAsWhite}
			{@const opponentPlayer = isPlayingAsWhite ? room.blackPlayer : room.whitePlayer}
			{@const selfTimer = isPlayingAsWhite ? whiteTimer : blackTimer}
			{@const opponentTimer = isPlayingAsWhite ? blackTimer : whiteTimer}
			{@const selfTakenPieces = isPlayingAsWhite ? game?.takenWhitePieces : game?.takenBlackPieces}
			{@const opponentTakenPieces = isPlayingAsWhite ? game?.takenBlackPieces : game?.takenWhitePieces}
			<Board
				board={board}
				isWhitePerspective={isPlayingAsWhite}
				potentialMoves={potentialMoves?.moves}
				onClickPiece={onClickPiece}
				selectedHexagon={selectedHexagon}
			/>
			<div class="side-table-wrapper">
				<PieceList pieces={opponentTakenPieces || []} />
				{#if opponentTimer}
					<div class="timer" class:timer-warn={opponentTimer < 15000}>
						{formatTimer(opponentTimer)}
					</div>
				{/if}
				<div class="side-table move-table-wrapper">
					<PlayerPanel player={opponentPlayer} isTurn={!isTurn} />
					<MoveList moveList={room.moveList} />
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
					<PlayerPanel player={selfPlayer} isTurn={isTurn || false} />
				</div>
				{#if selfTimer}
					<div class="timer" class:timer-warn={selfTimer < 15000}>
						{formatTimer(selfTimer)}
					</div>
				{/if}
				<PieceList pieces={selfTakenPieces || []} />
			</div>
		{:else if room}
			<div class="panel lobby">
				<div class="text-lg" style:margin-bottom="20px">
					Challenge to a game
				</div>
				<div class="text-sm" style:margin-bottom="30px">
					{formatTimeControl(timeControlIntMap[room.timeControl])}
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
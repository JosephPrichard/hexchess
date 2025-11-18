<script lang="ts">
import Board from '$lib/components/chess/Board.svelte';
import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
import type { ChessGame } from '$lib/api/messages';
import { createMoveState } from '$lib/state/move.svelte';
import { onMount } from 'svelte';
import Banner from '$lib/Banner.svelte';
import {
	mapHexagonList,
	getNewSelection,
	type Selection,
	isMoveValid, NoSelection, moveBoardPiece, setBoardTurn, placeBoardPiece, clearBoard, zeroGame
} from '$lib/services/chess.js';
import type { Hex } from '$lib/api/model';
import { getNotificationsContext } from '$lib/services/context';
import TrashcanIcon from '$lib/components/icons/TrashcanIcon.svelte';
import RedoIcon from '$lib/components/icons/RedoIcon.svelte';
import PieceEditor from '$lib/components/chess/PieceEditor.svelte';
import EditIcon from '$lib/components/icons/EditIcon.svelte';
import KingIcon from '$lib/components/icons/MoveIcon.svelte';
import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
import { boardToFenDyn, getInitialBoardDyn, makeMoveDyn } from '$lib/services/wasm';

const { addNotification } = getNotificationsContext();

let boardElement: HTMLElement | undefined = $state(undefined);

let moveState = createMoveState();
let mode: "EDIT" | "PLAY" = $state("EDIT");

let selection: Selection = $state(NoSelection);
let hoveringHex: Hex | undefined = $state(undefined);
let selectedPiece: number | undefined = $state(undefined);

let game: ChessGame | undefined = $state(undefined);
let fen: string = $state("");

function onSelectPiece(hex: Hex) {
	selection = getNewSelection(game, hex);
}

function onDeSelectPiece() {
	selection = NoSelection;
}

function handleSetBoardTurn(event: Event) {
	const turn = (event.target as HTMLSelectElement).value == "WHITE";
	game = setBoardTurn($state.snapshot(game?.board), turn)
}
async function onDropPieceSet(hex: Hex) {
	if (selectedPiece) {
		game = placeBoardPiece($state.snapshot(game?.board), hex, selectedPiece);
		fen = await boardToFenDyn(game?.board);
	}
}

function onClickClearBoard() {
	onDeSelectPiece();
	game = clearBoard($state.snapshot(game?.board));
}

async function onCopyFen() {
	await navigator.clipboard.writeText(fen);
	addNotification({ type: 'string', message: "Copied to clipboard!", isSuccess: true, duration: 2000 });
}

async function onPieceMove(from: Hex, to: Hex) {
	let isValid = true;
	if (mode == "PLAY") {
		isValid = isMoveValid(game, from, to);
	}
	const isSameHex = from.file == to.file && from.rank == to.rank;
	if (isSameHex || !isValid) {
		return;
	}

	const tempBoard = $state.snapshot(game?.board);

	if (mode == "EDIT") {
		game = moveBoardPiece(tempBoard, from, to);
	} else if (mode == "PLAY") {
		game = await makeMoveDyn(tempBoard, {from, to});
	}
	fen = await boardToFenDyn(game?.board);

	onDeSelectPiece();
}

async function setMode(newMode: 'EDIT' | 'PLAY') {
	const tempBoard = $state.snapshot(game?.board);

	if (mode == "EDIT") {
		game = await makeMoveDyn(tempBoard); // noop move, just calculate moves for curr board
	} else if (mode == "PLAY") {
		game = zeroGame(tempBoard);
	}

	selectedPiece = undefined;
	mode = newMode;
}

async function loadInitialGame() {
	game = zeroGame(await getInitialBoardDyn());
	fen = await boardToFenDyn(game.board);
}

onMount(loadInitialGame);

const draggable = $derived.by(() => mode == "PLAY" ? "turn" : "anyone");

const prevMove = $derived.by(() => {
	return !game?.moveList || game.moveList.length == 0
		? undefined
		: game.moveList[game.moveList.length - 1];
});

</script>
<svelte:head>
	<title>Sandbox - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-vertical-container" style="align-items: stretch;">
	<Board
		board={game?.board}
		bind:boardElement={boardElement}
		{draggable}
		isWhitePerspective={moveState.value.isWhitePerspective}
		potentialMoves={mode === "PLAY" ? mapHexagonList(selection.potentialMoves?.moves) : undefined}
		selectedHexagon={selection.hex}
		prevMove={prevMove}
		bind:hoveringHexagon={hoveringHex}
		onSelectPiece={onSelectPiece}
		onDeSelectPiece={onDeSelectPiece}
		onDropPiece={onPieceMove}
		onSetPiece={onDropPieceSet}
	/>
	<div class="side-bar">
		<select
			class="turn-selector"
			value={game?.board?.isWhiteTurn ? "WHITE" : "BLACK"}
			name="player-turn"
			onchange={handleSetBoardTurn}
			disabled={mode === "PLAY"}
		>
			<option value="WHITE">White to Play</option>
			<option value="BLACK">Black to Play</option>
		</select>
		<div class="piece-panels-container">
			<PieceEditor
				isWhitePerspective={moveState.value.isWhitePerspective}
				bind:selectedPiece={selectedPiece}
				bind:boardElement={boardElement}
				bind:hoveringHexagon={hoveringHex}
				onDropPiece={onDropPieceSet}
				isDisabled={mode === "PLAY"}
			/>
		</div>
		<div style:margin-top="10px"></div>
		{#if mode === "PLAY" }
			<button class="button-transparent side-bar-button"  onclick={() => setMode("EDIT")}>
				<EditIcon />
				<span class="button-text">
					Edit Board
				</span>
			</button>
		{:else}
			<button class="button-transparent side-bar-button" onclick={() => setMode("PLAY")}>
				<KingIcon />
				<span class="button-text">
					Make Moves
				</span>
			</button>
		{/if}
		<button class="button-transparent side-bar-button" onclick={moveState.flip}>
			<FlipIcon />
			<span class="button-text">
				Flip Board
			</span>
		</button>
		<button class="button-transparent side-bar-button" onclick={loadInitialGame}>
			<RedoIcon />
			<span class="button-text">
				Reset Board
			</span>
		</button>
		<button class="button-transparent side-bar-button" onclick={onClickClearBoard}>
			<TrashcanIcon />
			<span class="button-text">
				Clear Board
			</span>
		</button>
		<button class="button-transparent side-bar-button" onclick={onCopyFen}>
			<ClipboardIcon />
			<span class="button-text">
				Copy FEN
			</span>
		</button>
	</div>
</div>
<style>
	.button-text {
        user-select: none;
        -moz-user-select: none;
        -webkit-user-select: none;
	}

	.turn-selector {
        min-width: 165px;
		max-width: 165px;
        box-sizing: border-box;
		height: 40px;
        margin: 0 0 10px;
    }

    .piece-panels-container {
		display: flex;
		flex-direction: row;
		gap: 10px;
		margin-bottom: 10px;
	}

	.side-bar {
		width: 140px;
	}

	.side-bar-button {
		display: flex;
		flex-direction: row;
        align-items: center;
        gap: 10px;
		width: calc(100% - 15px);
	}
</style>
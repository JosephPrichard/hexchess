<script lang="ts">
import Board from '$lib/components/chess/Board.svelte';
import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
import { ChessBoard, type ChessGame } from '$lib/api/messages';
import { createMoveState } from '$lib/state/move.svelte';
import { onMount } from 'svelte';
import Banner from '$lib/Banner.svelte';
import {
	mapHexagonList,
	getNewSelection,
	type Selection,
	isMoveValid, NoSelection, moveBoardPiece, setBoardTurn, placeBoardPiece, clearBoard, newGame, defaultGame
} from '$lib/services/chess.js';
import type { Hex } from '$lib/api/model';
import { getNotificationsContext } from '$lib/services/context';
import TrashcanIcon from '$lib/components/icons/TrashcanIcon.svelte';
import RedoIcon from '$lib/components/icons/RedoIcon.svelte';
import PieceEditor from '$lib/components/chess/PieceEditor.svelte';
import EditIcon from '$lib/components/icons/EditIcon.svelte';
import KingIcon from '$lib/components/icons/MoveIcon.svelte';
import { boardToFenDyn, fenToGameDyn, getInitialBoardDyn, makeMoveDyn } from '$lib/services/wasm';
import MoveList from '$lib/components/chess/MoveList.svelte';

export interface SandboxProps {
	fen: string;
}

const { data: props }: { data: SandboxProps } = $props();

const { addNotification } = getNotificationsContext();

let boardElement: HTMLElement | undefined = $state(undefined);

let moveState = createMoveState();
let mode: "EDIT" | "PLAY" = $state("EDIT");

let selection: Selection = $state(NoSelection);
let hoveringHex: Hex | undefined = $state(undefined);
let selectedPiece: number | undefined = $state(undefined);

let fen = $state(props.fen);

let game: ChessGame | undefined = $state(undefined);

function onSelectPiece(hex: Hex) {
	selection = getNewSelection(game, hex);
}

function onDeSelectPiece() {
	selection = NoSelection;
}

async function handleSetBoardTurn(event: Event) {
	const turn = (event.target as HTMLSelectElement).value == "WHITE";
	game = setBoardTurn($state.snapshot(game?.board), turn)
	await gotoFen(game?.board);
}

async function onDropPieceSet(hex: Hex) {
	if (selectedPiece) {
		game = placeBoardPiece($state.snapshot(game?.board), hex, selectedPiece);
		await gotoFen(game?.board);
	}
}

function onClickClearBoard() {
	onDeSelectPiece();
	game = clearBoard($state.snapshot(game?.board));
}

async function gotoFen(board?: ChessBoard) {
	const f = await boardToFenDyn(board);
	if (!f) {
		return;
	}
	// await goto(`/sandbox?fen=${fen}`, { replaceState: true });
	fen = f;
	history.pushState({}, "", `/sandbox?fen=${fen}`);
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
		await gotoFen(game?.board);
	} else if (mode == "PLAY") {
		const tempGame = await makeMoveDyn(tempBoard, {from, to});
		await gotoFen(tempGame?.board); // update fen before the state so we can set the right game with move history after URL is updated
		tempGame.moveList = [...game?.moveList || [], ...tempGame.moveList]
		game = tempGame;
	}

	onDeSelectPiece();
}

async function setMode(newMode: 'EDIT' | 'PLAY') {
	const tempBoard = $state.snapshot(game?.board);

	if (mode == "EDIT") {
		game = await makeMoveDyn(tempBoard); // noop move, just calculate moves for curr board
	} else if (mode == "PLAY") {
		game = newGame(tempBoard);
	}

	selectedPiece = undefined;
	mode = newMode;
}

async function loadInitialGame() {
	game = newGame(await getInitialBoardDyn());
	await gotoFen(game?.board);
}

async function loadBoardFen(fenInput: string) {
	let isInitialGame = false;
	if (fenInput != "") {
		const results = await fenToGameDyn(fenInput);
		if (results.err) {
			addNotification({ type: 'string', message: results.err, isSuccess: false });
			isInitialGame = true;
		} else {
			game = results.game || defaultGame;
			await gotoFen(game?.board);
		}
	} else {
		isInitialGame = true;
	}
	if (isInitialGame) {
		await loadInitialGame();
	}
}

$effect(() => {
	loadBoardFen(props.fen);
});

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
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board
			board={game?.board}
			fen={fen}
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
			{#if mode === "PLAY"}
				<div class="side-table growing-box">
					<div class="side-table-header">
						<div class="turn-circle" class:turn-circle-green={Boolean(game?.board?.isWhiteTurn) !== moveState.value.isWhitePerspective}></div>
						{moveState.value.isWhitePerspective ? "Black's Turn" : "White's Turn"}
					</div>
					<MoveList moveList={game?.moveList || []} />
					<div class="side-table-header-bottom">
						<div class="turn-circle" class:turn-circle-green={Boolean(game?.board?.isWhiteTurn) === moveState.value.isWhitePerspective}></div>
						{moveState.value.isWhitePerspective ? "White's Turn" : "Black's Turn"}
					</div>
				</div>
			{:else}
				<div class="growing-box">
					<select
						class="turn-selector"
						value={game?.board?.isWhiteTurn ? "WHITE" : "BLACK"}
						name="player-turn"
						onchange={handleSetBoardTurn}
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
						/>
					</div>
				</div>
			{/if}
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
		</div>
	</div>
</div>
<style>
	.button-text {
        user-select: none;
        -moz-user-select: none;
        -webkit-user-select: none;
	}

	.turn-selector {
		width: 100%;
		/*min-width: 165px;*/
        box-sizing: border-box;
		height: 40px;
        margin: 0 0 10px;
        user-select: none;
        -moz-user-select: none;
        -webkit-user-select: none;
    }

    .piece-panels-container {
		display: flex;
		flex-direction: row;
		gap: 10px;
		margin-bottom: 10px;
	}

	.side-bar {
		width: 225px;
        display: flex;
        flex-direction: column;
        height: calc(100vh - 75px - 40px); /* Screen height minus the banner height minus the margin height */
	}

	.side-bar-button {
		display: flex;
		flex-direction: row;
        align-items: center;
        gap: 10px;
		width: calc(100% - 15px);
	}
</style>
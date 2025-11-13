<script lang="ts">
import Board from '$lib/components/chess/Board.svelte';
import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
import type { ChessGame } from '$lib/api/messages';
import services from '$lib/api/services';
import { createMoveState } from '$lib/state/move.svelte';
import { onMount } from 'svelte';
import Banner from '$lib/components/Banner.svelte';
import { mapHexagonList, handleSelectPiece, type Selection, handleDeSelectPiece, countPieces } from '$lib/utils/chess.js';
import type { Hexagon } from '$lib/api/model';
import { createMessage } from '$lib/utils/error';
import { getNotificationsContext } from '$lib/utils/context';
import TrashcanIcon from '$lib/components/icons/TrashcanIcon.svelte';
import RedoIcon from '$lib/components/icons/RedoIcon.svelte';
import { blackPieces, type BoardErr, boardErrMessages, pieces, whitePieces } from '$lib/utils/globals';
import PieceEditor from '$lib/components/chess/PieceEditor.svelte';

const { addNotification } = getNotificationsContext();

let mode: "EDIT" | "PLAY" = $state("EDIT");
let moveState = createMoveState();

let game: ChessGame | undefined = $state(undefined);

let selection: Selection = $state({
	potentialMoves: undefined,
	hex: undefined
});

function onSelectPiece(next: Hexagon) {
	if (!game) {
		return;
	}
	selection = handleSelectPiece(game, selection, next);
}

function onDeSelectPiece() {
	selection = handleDeSelectPiece();
}

function onClickClearBoard() {
	game = undefined;
}

async function loadInitialGame() {
	const [resp, err] = await services.postMakeMove();
	if (resp) {
		game = resp.game;
	} else {
		const message = 'Failed to load initial board: ' + createMessage(err);
		addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
	}
}

onMount(loadInitialGame);

const draggable = $derived.by(() => mode == "PLAY" ? "turn" : "anyone");

const errors = $derived.by(() => {
	const [pieceCount, kingCount] = countPieces(game?.board);

	const errors: BoardErr[] = [];
	if (pieceCount == 0) {
		errors.push('ERR_PIECES');
	}
	if (kingCount == 0) {
		errors.push('ERR_KINGS');
	}
	return errors;
});

</script>
<svelte:head>
	<title>Sandbox - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-vertical-container" style="align-items: stretch;">
	<Board
		board={game?.board}
		{draggable}
		isWhitePerspective={moveState.value.isWhitePerspective}
		potentialMoves={mode === "PLAY" ? mapHexagonList(selection.potentialMoves?.moves) : undefined}
		onSelectPiece={onSelectPiece}
		onDeSelectPiece={onDeSelectPiece}
		selectedHexagon={selection.hex}
	/>
	<div class="side-bar">
		<div class="piece-panels-container">
			<PieceEditor pieces={whitePieces}/>
			<PieceEditor pieces={blackPieces}/>
		</div>
		<div style:margin-top="10px"></div>
		<button class="button-transparent side-bar-button" style:padding-top="5px" onclick={moveState.flip}>
			<FlipIcon />
			<span>
				Flip Board
			</span>
		</button>
		<button class="button-transparent side-bar-button" style:padding-top="5px" onclick={loadInitialGame}>
			<RedoIcon />
			<span>
				Reset Board
			</span>
		</button>
		<button class="button-transparent side-bar-button" style:padding-top="5px" onclick={onClickClearBoard}>
			<TrashcanIcon />
			<span>
				Clear Board
			</span>
		</button>
		<div class="sandbox-options">
			<button class="sandbox-option-button" onclick={() => mode = "EDIT"} class:sandbox-option-active={mode === "EDIT"}>
				Edit
			</button>
			<button class="sandbox-option-button" onclick={() => mode = "PLAY"} class:sandbox-option-active={mode === "PLAY"}>
				Play
			</button>
		</div>
		{#if mode === "PLAY"}
			{#each errors as v}
				<div class="error-message"> {boardErrMessages[v]} </div>
			{/each}
		{/if}
	</div>
</div>
<style>
	.piece-panels-container {
		display: flex;
		flex-direction: row;
		gap: 10px;
		margin-bottom: 10px;
	}

	.error-message {
		font-size: 14px;
        margin-top: 10px;
        margin-bottom: 10px;
        border: 2px solid rgb(180, 50, 50);
        background-color: rgb(240, 150, 150);
        color: rgb(180, 50, 50);
        padding: 5px;
        border-radius: 5px;
	}

	.side-bar {
		width: 150px;
	}

	.side-bar-button {
		display: flex;
		flex-direction: row;
        align-items: center;
        gap: 10px;
		width: calc(100% - 15px);
	}

	.sandbox-options {
		margin: auto;
		margin-top: 10px;
		margin-bottom: 10px;
        display: flex;
        flex-direction: row;
		border-radius: 5px;
		border: 2px rgb(80, 80, 80) solid;
	}

	.sandbox-option-button {
		background-color: rgb(0, 0, 0, 0);
		font-weight: bold;
		border: none;
		color: #B4B4B4;
		padding: 8px;
		flex: 0.5;
		cursor: pointer;
	}

	.sandbox-options > button:first-child {
        padding-left: 12px;
	}

	.sandbox-options > button:last-child {
        padding-right: 12px;
	}

	.sandbox-option-active {
        background-color: rgb(80, 80, 80);
	}
</style>
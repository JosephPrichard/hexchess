<script lang="ts">
import Board from '$lib/components/chess/Board.svelte';
import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
import type { ChessGame } from '$lib/api/messages';
import services from '$lib/api/services';
import { createMoveState } from '$lib/state/move.svelte';
import { onMount } from 'svelte';
import Banner from '$lib/components/Banner.svelte';
import { mapHexagonList, handleSelectPiece, type Selection, handleDeSelectPiece } from '$lib/utils/chess.js';
import type { Hexagon } from '$lib/api/model';
import { createMessage } from '$lib/utils/error';
import { getNotificationsContext } from '$lib/utils/context';
import TrashcanIcon from '$lib/components/icons/TrashcanIcon.svelte';
import RedoIcon from '$lib/components/icons/RedoIcon.svelte';

const { addNotification } = getNotificationsContext();

let mode: "EDIT" | "VIEW" = $state("EDIT");
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

const draggable = $derived.by(() => mode == "VIEW" ? "turn" : "anyone");

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
		potentialMoves={mode === "VIEW" ? mapHexagonList(selection.potentialMoves?.moves) : undefined}
		onSelectPiece={onSelectPiece}
		onDeSelectPiece={onDeSelectPiece}
		selectedHexagon={selection.hex}
	/>
	<div class="side-bar">
		<div class="sandbox-options">
			<button class="sandbox-option-button" onclick={() => mode = "EDIT"} class:sandbox-option-active={mode === "EDIT"}>
				Edit
			</button>
			<button class="sandbox-option-button" onclick={() => mode = "VIEW"} class:sandbox-option-active={mode === "VIEW"}>
				View
			</button>
		</div>
		<div style:margin-top="10px"></div>
		<button title="Flip Board" class="button-transparent side-bar-button" style:padding-top="5px" onclick={moveState.flip}>
			<FlipIcon />
			<span>
				Flip Board
			</span>
		</button>
		<button title="Reset Board" class="button-transparent side-bar-button" style:padding-top="5px" onclick={loadInitialGame}>
			<RedoIcon />
			<span>
				Reset Board
			</span>
		</button>
		<button title="Clear Board" class="button-transparent side-bar-button" style:padding-top="5px" onclick={onClickClearBoard}>
			<TrashcanIcon />
			<span>
				Clear Board
			</span>
		</button>
	</div>
</div>
<style>
	.side-bar {
		width: 200px;
	}

	.side-bar-button {
		display: flex;
		flex-direction: row;
        align-items: center;
        gap: 10px;
		width: calc(100% - 15px);
	}

	.side-bar-button > div {
		text-align: center;
	}

	.sandbox-options {
		margin: auto;
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
		/*color: rgb(44, 44, 44);*/
	}
</style>
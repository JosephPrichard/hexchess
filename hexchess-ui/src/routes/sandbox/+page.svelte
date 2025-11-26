<script lang="ts">
	import Board from '$lib/components/chess/Board.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { ChessBoard, type ChessGame } from '$lib/api/messages';
	import { makeMoveState } from '$lib/state/move.svelte';
	import Banner from '$lib/Banner.svelte';
	import { clearBoard, defaultGame, makeGame, mapHexagons, moveBoardPiece, placeBoardPiece, removeBoardPiece, setBoardTurn } from '$lib/utils/chess.js';
	import type { Hex } from '$lib/api/model';
	import { getNotificationsContext } from '$lib/utils/context';
	import SmallTrashIcon from '$lib/components/icons/SmallTrashIcon.svelte';
	import RedoIcon from '$lib/components/icons/RedoIcon.svelte';
	import PieceEditor from '$lib/components/chess/PieceEditor.svelte';
	import EditIcon from '$lib/components/icons/EditIcon.svelte';
	import KingIcon from '$lib/components/icons/MoveIcon.svelte';
	import { boardToFenWasm, fenToGameWasm, getInitialGameWasm, getMoveNotationsWasm, getMovesWasm, makeMoveWasm } from '$lib/api/wasm';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import TurnWrapper from '$lib/components/chess/TurnWrapper.svelte';
	import { makeMoveSelectionState } from '$lib/state/selection.svelte';

	export interface SandboxProps {
	fen: string;
}

const { data: props }: { data: SandboxProps } = $props();

const { addNotification } = getNotificationsContext();

let boardElement: HTMLElement | undefined = $state(undefined);

const moveState = makeMoveState();
const selectionState = makeMoveSelectionState();

let mode: "edit" | "play" = $state("edit");
let isTrashcanSelect = $state(false);

let hoveringHex: Hex | undefined = $state(undefined);
let selectedPiece: number | undefined = $state(undefined);

let fen = $state(props.fen);

let game: ChessGame | undefined = $state(undefined);

async function onSelectPiece(hex: Hex) {
	if (isTrashcanSelect) {
		game = removeBoardPiece($state.snapshot(game?.board), hex);
		await setFen(game?.board);
		onDeSelectPiece();
	} else {
		selectionState.select(game, hex);
	}
}

function onDeSelectPiece() {
	selectionState.deSelect();
}

async function handleSetBoardTurn(event: Event) {
	const turn = (event.target as HTMLSelectElement).value == "WHITE";
	game = setBoardTurn(game?.board, turn)
	await setFen(game?.board);
}

async function onDropPieceSet(hex: Hex) {
	if (selectedPiece && !isTrashcanSelect) {
		const ret = placeBoardPiece($state.snapshot(game?.board), hex, selectedPiece);
		if (ret) {
			game = ret;
			await setFen(game?.board);
		}
	}
}

async function onClickClearBoard() {
	onDeSelectPiece();
	game = clearBoard($state.snapshot(game?.board));
	await setFen(game?.board);
}

async function setFen(board?: ChessBoard) {
	const f = await boardToFenWasm(board);
	if (!f) {
		return;
	}
	// await goto(`/sandbox?fen=${fen}`, { replaceState: true });
	fen = f;
	history.replaceState({}, "", `/sandbox?fen=${encodeURIComponent(fen)}`);
}

async function onPieceMove(from: Hex, to: Hex) {
	switch (mode) {
	case "edit":
		const ret = moveBoardPiece($state.snapshot(game?.board), from, to);
		if (ret) {
			game = ret;
			await setFen(game?.board);
		}
		onDeSelectPiece();
		break;
	case "play":
		const nextGame = await makeMoveWasm($state.snapshot(game), {from, to}, true);
		if (nextGame !== undefined) {
			await setFen(nextGame?.board); // update fen before the state so we can set the right game with move history after URL is updated
			game = nextGame;
			onDeSelectPiece();
		}
		break;
	}
}

async function setMode(newMode: "edit" | "play") {
	const board = $state.snapshot(game?.board);
	if (!board) {
		return
	}

	switch (mode) {
	case "edit":
		const nextGame = await getMovesWasm(board);
		if (nextGame !== undefined) {
			game = nextGame
		}
		break;
	case "play":
		game = makeGame(board);
		break;
	}

	selectedPiece = undefined;
	mode = newMode;
}

async function loadInitialGame() {
	game = await getInitialGameWasm();
	await setFen(game?.board);
	onDeSelectPiece();
}

async function loadBoardFen(fenInput: string) {
	let isInitialGame = false;
	if (fenInput != "") {
		const result = await fenToGameWasm(fenInput);
		if (typeof result === "string") {
			addNotification({ type: 'string', message: result, isSuccess: false });
			isInitialGame = true;
		} else {
			game = result || defaultGame;
			await setFen(game?.board);
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

const draggable = $derived.by(() => mode == "play" ? "turn" : "anyone");

const prevMove = $derived.by(() => (
	!game?.moves || game.moves.length == 0
		? undefined
		: game.moves[game.moves.length - 1]
));

const potentialMoves = $derived.by(() => mode === "play" ? mapHexagons(selectionState.value.potentialMoves?.moves) : undefined);

const awaitingNotList = $derived.by(async () => await getMoveNotationsWasm(game?.moves));

</script>
<svelte:head>
	<title>Sandbox - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board
			board={game?.board}
			fen={true}
			bind:boardElement={boardElement}
			{draggable}
			isWhitePerspective={moveState.value.isWhitePerspective}
			potentialMoves={potentialMoves}
			selectedHexagon={selectionState.value.hex}
			prevMove={prevMove}
			bind:hoveringHexagon={hoveringHex}
			onSelectPiece={onSelectPiece}
			onDeSelectPiece={onDeSelectPiece}
			onDropPiece={onPieceMove}
			onSetPiece={onDropPieceSet}
		/>
		<div class="side-bar side-table-capped" class:side-bar-short={mode === "edit"} class:side-bar-long={mode === "play"}>
			{#if mode === "play"}
				<div class="side-table growing-box sandbox-display">
					<TurnWrapper isWhitePerspective={moveState.value.isWhitePerspective} isWhiteTurn={game?.board?.isWhiteTurn}>
						{#await awaitingNotList then notList}
							<MoveList moveList={notList} />
						{/await}
					</TurnWrapper>
				</div>
			{:else}
				<div class="growing-box sandbox-display">
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
							bind:isTrashSelector={isTrashcanSelect}
						/>
					</div>
				</div>
			{/if}
			<div class="sandbox-buttons">
				{#if mode === "play" }
					<button class="button-transparent side-bar-button"  onclick={() => setMode("edit")}>
						<EditIcon />
						<span class="button-text">
							Edit Board
						</span>
					</button>
				{:else}
					<button class="button-transparent side-bar-button" onclick={() => setMode("play")}>
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
					<SmallTrashIcon />
					<span class="button-text">
						Clear Board
					</span>
				</button>
			</div>
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
		/*width: 100%;*/
		min-width: 165px;
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
		width: 165px;
        display: flex;
        flex-direction: column;
		gap: 10px;
	}

    .side-bar-short {
        width: 165px;
        margin-right: calc(225px - 165px);
    }

    .side-bar-long {
        width: 225px;
    }

	.side-bar-button {
		display: flex;
		flex-direction: row;
        align-items: center;
        gap: 10px;
		width: calc(100% - 15px);
	}

	.sandbox-display {
		flex: 0.75;
	}

	.sandbox-buttons {
		flex: 0.25;
	}
</style>
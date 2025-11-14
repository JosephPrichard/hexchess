<script lang="ts">
import Board from '$lib/components/chess/Board.svelte';
import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
import type { ChessGame } from '$lib/api/messages';
import services from '$lib/api/services';
import { createMoveState } from '$lib/state/move.svelte';
import { onMount } from 'svelte';
import Banner from '$lib/components/Banner.svelte';
import {
	mapHexagonList,
	handleSelectPiece,
	type Selection,
	handleDeSelectPiece,
	whitePieces,
	blackPieces,
	defaultGame, pieces, mapHexagon
} from '$lib/utils/chess.js';
import type { Hex } from '$lib/api/model';
import { createMessage } from '$lib/utils/error';
import { getNotificationsContext } from '$lib/utils/context';
import TrashcanIcon from '$lib/components/icons/TrashcanIcon.svelte';
import RedoIcon from '$lib/components/icons/RedoIcon.svelte';
import PieceEditor from '$lib/components/chess/PieceEditor.svelte';

const { addNotification } = getNotificationsContext();

let mode: "EDIT" | "PLAY" = $state("EDIT");
let moveState = createMoveState();

let game: ChessGame | undefined = $state(undefined);

let selection: Selection = $state({ potentialMoves: undefined, hex: undefined });

async function setModePlay() {
	const [resp, err] = await services.postMakeMove({ board: game?.board });
	if (resp) {
		game = resp.game;
		mode = "PLAY";
	} else {
		const message = 'Failed to load board moves: ' + createMessage(err);
		addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
	}
}

function onSelectPiece(next: Hex) {
	if (!game)
		return;
	selection = handleSelectPiece(game, selection, next);
}

function handleSetBoardTurn(event: Event) {
	const target = event.target as HTMLSelectElement;
	const turn = target.value == "WHITE";
	if (game?.board) {
		const nextGame = {
			...defaultGame,
			board: structuredClone($state.snapshot(game?.board)),
		};
		nextGame.board.isWhiteTurn = turn;
		game = nextGame;
	}
}

function onDeSelectPiece() {
	selection = handleDeSelectPiece();
}

function isMoveValid(from: Hex, to: Hex) {
	if (mode == "EDIT")
		return true;
	const currMoves = (game?.board?.isWhiteTurn ?
		game.whiteMoves :
		game?.blackMoves)
		|| [];
	const pieceMoves = currMoves.find((hex) =>
		hex.fromFile == from.file && hex.fromRank == from.rank)?.moves || [];
	const index = pieceMoves.findIndex((hex) => {
		const mh = mapHexagon(hex);
		return mh.file == to.file && mh.rank == to.rank
	});
	return index >= 0;
}

function onDropPiece(from: Hex, to: Hex, piece: number) {
	if (!isMoveValid(from, to))
		return;
	if (mode == "EDIT") {
		const nextGame = {
			...defaultGame,
			board: structuredClone($state.snapshot(game?.board)),
		};
		if (nextGame?.board) {
			nextGame.board.file[to.file].pieces[to.rank] = piece;
			nextGame.board.file[from.file].pieces[from.rank] = pieces.empty;
		}
		game = nextGame;
	} else {
		(async() => {
			const [resp, err] = await services.postMakeMove({
				board: game?.board,
				move: { piece: piece, fromFile: from.file, fromRank: from.rank, toFile: to.file, toRank: to.rank }
			});
			if (resp) {
				game = resp.game;
			} else {
				const message = 'Failed to load board move: ' + createMessage(err);
				addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
			}
		})()
	}
}

function onClickClearBoard() {
	game = undefined;
}

async function loadInitialGame() {
	const [resp, err] = await services.postMakeMove({});
	if (resp) {
		game = resp.game;
	} else {
		const message = 'Failed to load initial board: ' + createMessage(err);
		addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
	}
}

onMount(loadInitialGame);

const draggable = $derived.by(() => mode == "PLAY" ? "turn" : "anyone");

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
		selectedHexagon={selection.hex}
		onSelectPiece={onSelectPiece}
		onDeSelectPiece={onDeSelectPiece}
		onDropPiece={onDropPiece}
	/>
	<div class="side-bar">
		<div class="sandbox-options-list">
			<select
				value={game?.board?.isWhiteTurn ? "WHITE" : "BLACK"}
				name="player-turn" class="turn-selector"
				onchange={handleSetBoardTurn}
				disabled={mode === "PLAY"}
			>
				<option value="WHITE">White to Play</option>
				<option value="BLACK">Black to Play</option>
			</select>
			<div class="sandbox-options-switch">
				<div class="sandbox-options-title">
					Sandbox Mode
				</div>
				<div class="sandbox-options">
					<button class="sandbox-option-button" onclick={() => mode = "EDIT"} class:sandbox-option-active={mode === "EDIT"}>
						Edit
					</button>
					<button class="sandbox-option-button" onclick={setModePlay} class:sandbox-option-active={mode === "PLAY"}>
						Play
					</button>
				</div>
			</div>
		</div>
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
	</div>
</div>
<style>
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
		width: 165px;
	}

	.side-bar-button {
		display: flex;
		flex-direction: row;
        align-items: center;
        gap: 10px;
		width: calc(100% - 15px);
	}

	.sandbox-options-switch {
		height: 75px;
	}

	.sandbox-options-list {
		margin-top: 10px;
		margin-bottom: 10px;
	}

	.sandbox-options-title {
		font-size: 14px;
		padding: 1px;
	}

	.sandbox-options {
        margin: 5px auto;
        display: flex;
        flex-direction: row;
		border-radius: 5px;
		border: 2px rgb(80, 80, 80) solid;
	}

	.sandbox-option-button {
		background-color: rgb(0, 0, 0, 0);

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
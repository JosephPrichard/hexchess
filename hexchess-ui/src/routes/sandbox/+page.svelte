<script lang="ts">
	import Board from '$lib/components/chess/Board.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { ChessBoard } from '$lib/pb/messages';
	import { makeMoveState } from '$lib/state/move.svelte';
	import { deserializeHexList, isLastRank } from '$lib/utils/chess.js';
	import type { Hex } from '$lib/api/models';
	import { getNotificationsContext } from '$lib/utils/context';
	import SmallTrashIcon from '$lib/components/icons/SmallTrashIcon.svelte';
	import RedoIcon from '$lib/components/icons/RedoIcon.svelte';
	import PieceEditor from '$lib/components/chess/PieceEditor.svelte';
	import EditIcon from '$lib/components/icons/EditIcon.svelte';
	import KingIcon from '$lib/components/icons/MoveIcon.svelte';
	import { boardToFenWasm, fenToGameWasm, getInitialGameWasm, getMoveNotationsWasm, getMovesWasm, makeMoveWasm } from '$lib/api/wasm';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import TurnWrapper from '$lib/components/chess/TurnWrapper.svelte';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import Dropdown from '$lib/components/util/Dropdown.svelte';
	import { makeGameState } from '$lib/state/game.svelte';
	import Banner from '$lib/Banner.svelte';

	export interface SandboxProps {
		fen: string;
	}

	const { data: props }: { data: SandboxProps } = $props();

	const { addNotification } = getNotificationsContext();

	let boardElement: HTMLElement | undefined = $state(undefined);

	const moveState = makeMoveState();
	const selectionState = makeSelectionState();
	const gameState = makeGameState();

	let mode: "edit" | "play" = $state("edit");
	let isTrashcanSelect = $state(false);

	let hoveringHex: Hex | undefined = $state(undefined);
	let selectedPiece: number | undefined = $state(undefined);

	let fen = $state(props.fen);

	async function onSelectPiece(hex: Hex) {
		const game = gameState.value.game;
		if (isTrashcanSelect) {
			gameState.removePiece(hex);
			onDeSelectPiece();
		} else {
			selectionState.select(game, hex);
		}
	}

	function onDeSelectPiece() {
		selectionState.deSelect();
	}

	async function handleSetBoardTurn(value: string) {
		const turn = value == "WHITE";
		gameState.setTurn(turn)
	}

	async function onDropPieceSet(hex: Hex) {
		if (selectedPiece && !isTrashcanSelect) {
			gameState.placePiece(hex, selectedPiece);
		}
	}

	async function onClickClearBoard() {
		onDeSelectPiece();
		gameState.clear();
	}

	async function onPieceMove(from: Hex, to: Hex) {
		const game = gameState.value.game;
		switch (mode) {
		case "edit":
			gameState.movePiece(from, to);
			break;
		case "play":
			if (isLastRank(to)) {
				gameState.movePiece(from, to);
				gameState.setPromotion({from, to});
			} else {
				const next = await makeMoveWasm($state.snapshot(game), {from, to, promotion: 0});
				if (next !== undefined) {
					gameState.setGame(next);
				}
			}
			break;
		}
		onDeSelectPiece();
	}

	async function onCompletePromotion(promotedPiece?: number) {
		if (gameState.value.promotion === undefined) {
			return;
		}
		if (promotedPiece !== undefined) {
			const move = { from: gameState.value.promotion.from, to: gameState.value.promotion.to, promotion: promotedPiece };
			const next = await makeMoveWasm($state.snapshot(gameState.value.prevGame), move);
			if (next !== undefined) {
				gameState.setGame(next);
			} else {
				gameState.revert();
			}
		} else {
			gameState.revert();
		}
		gameState.setPromotion(undefined);
	}

	async function setMode(newMode: "edit" | "play") {
		const game = gameState.value.game;
		const board = $state.snapshot(game?.board);
		if (!board) {
			return
		}
		const next = await getMovesWasm(board);
		if (next !== undefined) {
			gameState.setGame(next);
		}
		selectedPiece = undefined;
		mode = newMode;
	}

	async function loadInitialGame() {
		gameState.setGame(await getInitialGameWasm());
		onDeSelectPiece();
	}

	$effect(() => {
		(async (fenInput: string) => {
			let isInitialGame = false;
			if (fenInput != "") {
				const result = await fenToGameWasm(fenInput);
				if (typeof result === "string") {
					addNotification({ type: 'string', message: result, isSuccess: false });
					isInitialGame = true;
				} else {
					gameState.setGame(result);
				}
			} else {
				isInitialGame = true;
			}
			if (isInitialGame) {
				await loadInitialGame();
			}
		})(props.fen)
	});

	$effect(() => {
		const game = gameState.value.game;
		(async (board?: ChessBoard) => {
			const f = await boardToFenWasm(board);
			if (!f) {
				return;
			}
			fen = f;
			history.replaceState({}, "", `/sandbox?fen=${encodeURIComponent(fen)}`);
		})(game?.board)
	});

	const draggable = $derived.by(() => mode == "play" ? "turn" : "anyone");

	const prevMove = $derived.by(() => {
		const game = gameState.value.game;
		return !game?.moves || game.moves.length == 0
			? undefined
			: game.moves[game.moves.length - 1]
	});

	const potentialMoves = $derived.by(() => mode === "play" ? deserializeHexList(selectionState.value.potentialMoves?.moves) : undefined);

	const awaitingNotList = $derived.by(async () => await getMoveNotationsWasm(gameState.value.game?.moves));
</script>
<svelte:head>
	<title>Sandbox - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board
			board={gameState.value.game?.board}
			isWhitePerspective={moveState.value.isWhitePerspective}
			fen={true}
			bind:boardElement={boardElement}
			draggable={draggable}
			selected={selectionState.value.hex}
			promotion={gameState.value.promotion}
			bind:hovering={hoveringHex}
			prevMove={prevMove}
			potentialMoves={potentialMoves}
			onSelectPiece={onSelectPiece}
			onDeSelectPiece={onDeSelectPiece}
			onDropPiece={onPieceMove}
			onSetPiece={onDropPieceSet}
			onCompletePromotion={onCompletePromotion}
		/>
		<div class="side-bar side-table-capped" class:side-bar-short={mode === "edit"} class:side-bar-long={mode === "play"}>
			{#if mode === "play"}
				<div class="side-table growing-box sandbox-display">
					<TurnWrapper isWhitePerspective={moveState.value.isWhitePerspective} isWhiteTurn={gameState.value.game?.board?.isWhiteTurn}>
						{#await awaitingNotList then notList}
							<MoveList moveList={notList} />
						{/await}
					</TurnWrapper>
				</div>
			{:else}
				<div class="growing-box sandbox-display">
					<Dropdown
						options={[{label: "White's Turn", value: "WHITE"}, {label: "Black's Turn", value: "BLACK"}]}
						selected={gameState.value.game?.board?.isWhiteTurn ? "WHITE" : "BLACK"} 
						onChange={handleSetBoardTurn}
					/>
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
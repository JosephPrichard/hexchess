<script lang="ts">
	import Board from '$lib/components/chess/Board.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { isPromotion } from '$lib/service/chess';
	import type { Hex } from '$lib/api/models';
	import SmallTrashIcon from '$lib/components/icons/SmallTrashIcon.svelte';
	import RedoIcon from '$lib/components/icons/RedoIcon.svelte';
	import PieceEditor from '$lib/components/chess/PieceEditor.svelte';
	import EditIcon from '$lib/components/icons/EditIcon.svelte';
	import KingIcon from '$lib/components/icons/KingIcon.svelte';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import { makeSelectionState } from '$lib/state/selection.svelte';
	import Dropdown from '$lib/components/util/Dropdown.svelte';
	import { makeSandboxState } from '$lib/state/sandbox.svelte.js';
	import Banner from '$lib/Banner.svelte';
	import { CancelPromotion, type BadPromotionType, type Promotion } from '$lib/components/chess/types';
	import { wasm } from '$lib/api/wasm';
	import type { ChessBoard } from '$lib/pb/messages';

	export interface SandboxProps {
		fen: string;
	}

	const { data: props }: { data: SandboxProps } = $props();

	let boardElement: HTMLElement | undefined = $state(undefined);

	const selection = makeSelectionState();
	const sandbox = makeSandboxState();

	const game = $derived(sandbox.state.game);
	const isWhiteTurn = $derived(game?.board?.isWhiteTurn);
	const moves = $derived(game?.moves || []);
	const moveIndex = $derived(moves.length - 1);

	let mode: "edit" | "play" = $state("edit");
	let isTrashcanEdit = $state(false);
	let fen = $state(props.fen);

	let hoveringHex: Hex | undefined = $state(undefined);
	let editPiece: number | undefined = $state(undefined);

	let isWhitePerspective = $state(true);

	const flip = () => isWhitePerspective = !isWhitePerspective;

	async function onSelectPiece(hex: Hex) {
		if (isTrashcanEdit && mode === "edit") {
			sandbox.removePiece(hex);
			onDeSelectPiece();
		} else {
			selection.select(game, hex);
		}
	}

	const onDeSelectPiece = () => selection.deSelect();

	const handleSetBoardTurn = (value: string) => sandbox.setTurn(value == "WHITE");

	async function onDropPieceSet(hex: Hex) {
		if (editPiece && !isTrashcanEdit) sandbox.placePiece(hex, editPiece);
	}

	async function onClickClearBoard() {
		onDeSelectPiece();
		sandbox.clear();
	}

	async function onPieceMove(from: Hex, to: Hex) {
		switch (mode) {
		case "edit":
			onEditMove(from, to);
			break;
		case "play":
			await onPlayMove(from, to);
			break;
		}
	}

	function onEditMove(from: Hex, to: Hex) {
		sandbox.movePiece(from, to);
		onDeSelectPiece();
	}

	async function onPlayMove(from: Hex, to: Hex) {
		console.log(to);
		if (isPromotion(game, from, to)) {
			sandbox.setPromotion({ from, to });
			onDeSelectPiece();
		} else {
			const didMove = await sandbox.makeMove({ from, to, promotion: 0 });
			if (!didMove) return;
			onDeSelectPiece();
		}
	}

	async function onCompletePromotion(promotion: Promotion | BadPromotionType) {
		if (sandbox.state.promotion === undefined) return; // never trigger a promotion if there is not an active promotion
		if (promotion === CancelPromotion) {
			sandbox.revertPromotion();
		} else {
			const didMove = await sandbox.makeMove({ from: sandbox.state.promotion.from, to: sandbox.state.promotion.to, promotion: promotion.kind });
			if (didMove) {
				onDeSelectPiece();
			}
		}
	}

	async function setMode(newMode: "edit" | "play") {
		mode = newMode;
		editPiece = undefined;
		onDeSelectPiece();
		await sandbox.initNewBoard();
	}

	async function onUpdateFen(fenInput: string) {
		const game = await wasm.fenToGame(fenInput);
		if (!game) return;

		sandbox.setGame(game);
		await boardToFenURL(game.board);
	}

	async function gameFromFenURL(fenInput: string) {
		let isInitialGame = false;
		if (fenInput != "") {
			const game = await wasm.fenToGame(fenInput);
			if (game) sandbox.setGame(game);
		} else {
			isInitialGame = true;
		}
		if (isInitialGame) {
			await sandbox.setInitialGame();
		}
	}

	async function boardToFenURL(board?: ChessBoard) {
		const f = await wasm.boardToFen(board);
		if (!f) return;
		fen = f;
		history.replaceState({}, "", `/sandbox?fen=${encodeURIComponent(fen)}`)
	}

	$effect(() => {
		gameFromFenURL(props.fen);
	});

	$effect(() => {
		boardToFenURL(game?.board);
	});

	const prevMove = $derived.by(() => {
		if (!moveIndex) return moves.at(-1);
		return moves?.[moveIndex]
	});

	const draggable = $derived.by(() => mode === "play" ? "turn" : "anyone");
	const potentialMoves = $derived.by(() => mode === "play" ? selection.getPotentialMoves() : undefined);
	const awaitingNotList = $derived.by(async () => await wasm.getMoveNotations(game?.moves));
</script>
<svelte:head>
	<title>Sandbox - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board
			board={game?.board}
			isWhitePerspective={isWhitePerspective}
			fen={true}
			onChangeFen={onUpdateFen}
			bind:boardElement={boardElement}
			draggable={draggable}
			selected={selection.state.hex}
			promotion={sandbox.state.promotion}
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
				<div class="side-table growing-box">
					<div class="side-table-header">
						<div class="turn-circle" class:turn-circle-green={Boolean(isWhiteTurn) !== isWhitePerspective}></div>
						{isWhitePerspective ? "Black's Turn" : "White's Turn"}
					</div>
					{#await awaitingNotList then notList}
						<MoveList moveList={notList} selectedMoveIndex={moveIndex} />
					{/await}
					<div class="side-table-header-bottom side-table-header-bottom-shadow">
						<div class="turn-circle" class:turn-circle-green={Boolean(isWhiteTurn) === isWhitePerspective}></div>
						{isWhitePerspective ? "White's Turn" : "Black's Turn"}
					</div>
				</div>
			{:else}
				<div class="growing-box sandbox-display">
					<Dropdown
						options={[{label: "White's Turn", value: "WHITE"}, {label: "Black's Turn", value: "BLACK"}]}
						selected={game?.board?.isWhiteTurn ? "WHITE" : "BLACK"}
						onChange={handleSetBoardTurn}
					/>
					<div class="piece-panels-container">
						<PieceEditor
							isWhitePerspective={isWhitePerspective}
							bind:selectedPiece={editPiece}
							bind:boardElement={boardElement}
							bind:hoveringHexagon={hoveringHex}
							onDropPiece={onDropPieceSet}
							bind:isTrashSelector={isTrashcanEdit}
						/>
					</div>
				</div>
			{/if}
			<div class="sandbox-buttons">
				{#if mode === "play" }
					<button class="button button-light-grey side-bar-button"  onclick={() => setMode("edit")}>
						<EditIcon />
						<span class="button-text">
							Edit Board
						</span>
					</button>
				{:else}
					<button class="button button-small-green side-bar-button" onclick={() => setMode("play")}>
						<KingIcon />
						<span class="button-text">
							Load Board
						</span>
					</button>
				{/if}
				<button class="button button-light-grey side-bar-button" onclick={flip}>
					<FlipIcon color="#D2D2D2"/>
					<span class="button-text">
						Flip Board
					</span>
				</button>
				<button class="button button-light-grey side-bar-button" onclick={sandbox.setInitialGame}>
					<RedoIcon />
					<span class="button-text">
						Reset Board
					</span>
				</button>
				<button class="button button-small-red side-bar-button" onclick={onClickClearBoard}>
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
        width: calc(70px * 3);
        display: flex;
        flex-direction: column;
		gap: 10px;
	}

	.side-bar-button {
		display: flex;
		flex-direction: row;
        align-items: center;
        justify-content: center;
        padding: 5px 0;
		margin-top: 10px;
		margin-bottom: 10px;
        gap: 5px;
		width: 100%;
	}

	.sandbox-display {
		flex: 0.75;
	}

	.sandbox-buttons {
		flex: 0.25;
	}
</style>
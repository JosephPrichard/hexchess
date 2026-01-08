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
	import { makeSandboxState } from '$lib/state/game.svelte';
	import Banner from '$lib/Banner.svelte';
	import { BadPromotion, type BadPromotionType } from '$lib/components/chess/piece';

	export interface SandboxProps {
		fen: string;
	}

	const { data: props }: { data: SandboxProps } = $props();

	let boardElement: HTMLElement | undefined = $state(undefined);

	const move = makeMoveState();
	const selection = makeSelectionState();
	const sandbox = makeSandboxState();

	let mode: "edit" | "play" = $state("edit");
	let isTrashcanSelect = $state(false);

	let hoveringHex: Hex | undefined = $state(undefined);
	let selectedPiece: number | undefined = $state(undefined);

	let fen = $state(props.fen);

	async function onSelectPiece(hex: Hex) {
		const game = sandbox.state.game;
		if (isTrashcanSelect) {
			sandbox.removePiece(hex);
			onDeSelectPiece();
		} else {
			selection.select(game, hex);
		}
	}

	function onDeSelectPiece() {
		selection.deSelect();
	}

	async function handleSetBoardTurn(value: string) {
		const turn = value == "WHITE";
		sandbox.setTurn(turn)
	}

	async function onDropPieceSet(hex: Hex) {
		if (selectedPiece && !isTrashcanSelect) {
			sandbox.placePiece(hex, selectedPiece);
		}
	}

	async function onClickClearBoard() {
		onDeSelectPiece();
		sandbox.clear();
	}

	async function onPieceMove(from: Hex, to: Hex) {
		const game = sandbox.state.game;
		switch (mode) {
		case "edit":
			sandbox.movePiece(from, to);
			break;
		case "play":
			if (isLastRank(to)) {
				sandbox.movePiece(from, to);
				sandbox.setPromotion({from, to});
			} else {
				const next = await makeMoveWasm($state.snapshot(game), {from, to, promotion: 0});
				if (next !== undefined) {
					sandbox.setGame(next);
				}
			}
			break;
		}
		onDeSelectPiece();
	}

	async function onCompletePromotion(promotedPiece: number | BadPromotionType) {
		if (sandbox.state.promotion === undefined) {
			return;
		}
		if (promotedPiece === BadPromotion) {
			sandbox.revert();
		} else {
			const move = { from: sandbox.state.promotion.from, to: sandbox.state.promotion.to, promotion: promotedPiece };
			const next = await makeMoveWasm($state.snapshot(sandbox.state.prevGame), move);
			if (next !== undefined) {
				sandbox.setGame(next);
			} else {
				sandbox.revert();
			}
		}
		sandbox.setPromotion(undefined);
	}

	async function setMode(newMode: "edit" | "play") {
		const game = sandbox.state.game;
		const board = $state.snapshot(game?.board);
		if (!board) {
			return
		}
		const next = await getMovesWasm(board);
		if (next !== undefined) {
			sandbox.setGame(next);
		}
		selectedPiece = undefined;
		mode = newMode;
	}

	async function loadInitialGame() {
		sandbox.setGame(await getInitialGameWasm());
		onDeSelectPiece();
	}

	async function boardFromFenURL(fenInput: string) {
		let isInitialGame = false;
		if (fenInput != "") {
			const game = await fenToGameWasm(fenInput);
			if (game) {
				sandbox.setGame(game);
			}
		} else {
			isInitialGame = true;
		}
		if (isInitialGame) {
			await loadInitialGame();
		}
	}

	async function fenURLFromBoard(board?: ChessBoard) {
		const f = await boardToFenWasm(board);
		if (!f) {
			return;
		}
		fen = f;
		history.replaceState({}, "", `/sandbox?fen=${encodeURIComponent(fen)}`);
	}

	async function onChangeFen(fenInput: string) {
		const game = await fenToGameWasm(fenInput);
		if (game) {
			sandbox.setGame(game);
			await fenURLFromBoard(game.board);
		}
	}

	$effect(() => {
		boardFromFenURL(props.fen);
	});

	$effect(() => {
		const game = sandbox.state.game;
		fenURLFromBoard(game?.board)
	});

	const draggable = $derived.by(() => mode == "play" ? "turn" : "anyone");
	const potentialMoves = $derived.by(() => mode === "play" ? selection.getPotentialMoves() : undefined);
	const awaitingNotList = $derived.by(async () => await getMoveNotationsWasm(sandbox.state.game?.moves));
</script>
<svelte:head>
	<title>Sandbox - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board
			board={sandbox.state.game?.board}
			isWhitePerspective={move.state.isWhitePerspective}
			fen={true}
			onChangeFen={onChangeFen}
			bind:boardElement={boardElement}
			draggable={draggable}
			selected={selection.state.hex}
			promotion={sandbox.state.promotion}
			bind:hovering={hoveringHex}
			prevMove={sandbox.getPrevMove()}
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
					<TurnWrapper isWhitePerspective={move.state.isWhitePerspective} isWhiteTurn={sandbox.state.game?.board?.isWhiteTurn}>
						{#await awaitingNotList then notList}
							<MoveList moveList={notList} />
						{/await}
					</TurnWrapper>
				</div>
			{:else}
				<div class="growing-box sandbox-display">
					<Dropdown
						options={[{label: "White's Turn", value: "WHITE"}, {label: "Black's Turn", value: "BLACK"}]}
						selected={sandbox.state.game?.board?.isWhiteTurn ? "WHITE" : "BLACK"}
						onChange={handleSetBoardTurn}
					/>
					<div class="piece-panels-container">
						<PieceEditor
							isWhitePerspective={move.state.isWhitePerspective}
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
				<button class="button-transparent side-bar-button" onclick={move.flip}>
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
<script lang="ts">
	import type { ChessBoard, PieceMove } from '$lib/pb/messages';
	import Piece, { type PromotionKind } from './Piece.svelte';
	import type { Hex } from '$lib/api/models';
	import { defaultBoard, findKeyedPieces, hexEq, isPieceBlack, isPieceWhite, pieces, type PlacedPiece, ranksPerFile } from '$lib/utils/chess.js';
	import { getLeft, getTop, hexHeight, hexWidth, selectedColor, colors, colorsOffset, findHex, highlightedColor, hoveringColor } from '$lib/components/chess/render';
	import Fen from '$lib/components/chess/Fen.svelte';
	import { browser } from '$app/environment';
	import { boardToFenWasm } from '$lib/api/wasm';
	import type { Promotion } from '$lib/state/game.svelte';

	export interface BoardProps {
		board?: ChessBoard;
		fen?: string | boolean;
		boardElement?: HTMLElement;
		draggable?: "turn" | "anyone" | "none";
		isWhitePerspective: boolean;
		hovering?: Hex;
		selected?: Hex;
		promotion?: Promotion;
		prevMove?: PieceMove;
		potentialMoves?: Hex[];
		onSelectPiece?: (hex: Hex) => void;
		onDeSelectPiece?: (hex: Hex) => void;
		onDropPiece?: (from: Hex, to: Hex) => void;
		onSetPiece?: (hex: Hex) => void;
		onCompletePromotion?: (promotedPiece?: number) => void;
	}

	let { 
		board, 
		fen,
		isWhitePerspective, 
		boardElement = $bindable(),
		draggable,
		selected,
		hovering = $bindable(),
		promotion,
		potentialMoves,
		prevMove, 
		onSelectPiece,
		onDeSelectPiece,
		onDropPiece,
		onSetPiece,
		onCompletePromotion 
	}: BoardProps = $props();

	let prevPieces: [number, PlacedPiece][] | undefined = undefined;

	function onDragBoardPiece(x: number, y: number) {
		hovering = findHex(boardElement, isWhitePerspective, x, y);
	}

	function onDropBoardPiece(from: Hex, x: number, y: number, piece: number) {
		hovering = undefined;
		const hex = findHex(boardElement, isWhitePerspective, x, y);
		if (!hex)
			return
		if (draggable === "anyone" ||
			draggable === "turn" && isPieceWhite(piece) && board?.isWhiteTurn ||
			draggable === "turn" && isPieceBlack(piece) && !board?.isWhiteTurn
		) {
			onDropPiece?.(from, hex);
		}
	}

	function onClickHexagon(file: number, rank: number) {
		if (selected) {
			onDropPiece?.(selected, {file, rank});
		} else {
			onSetPiece?.({file, rank});
		}
	}

	function makeMoveKey(file: number, rank: number) {
		return file + "," + rank;
	}

	const pmMap = $derived.by(() => {
		const pmm: Map<string, boolean> = new Map();
		if (potentialMoves) {
			for (const move of potentialMoves) {
				pmm.set(makeMoveKey(move.file, move.rank), true);
			}
		}
		return pmm;
	});

	const [boardState, boardPieces] = $derived.by(() => {
		const boardState = board ?? defaultBoard;
		if (browser) {
			const boardPieces = findKeyedPieces(boardState, prevPieces)
			prevPieces = boardPieces;
			// console.log("pieceEntries", boardPieces);
			return [boardState, boardPieces];
		}
		return [boardState, []];
	});

	const awaitingFen = $derived.by(async () => {
		if (typeof fen === "string") {
			return Promise.resolve(fen);
		} else if (fen && board && browser) {
			return boardToFenWasm(board);
		}
	});

	function pickTileBgColor(
		{isPrevMove, isSelected, isMoveTarget, isHovering}:
		{isPrevMove?: boolean, isSelected?: boolean, isMoveTarget?: boolean, isHovering?: boolean}
	) {
		if (isPrevMove) {
			return highlightedColor
		} else if (isSelected) {
			return selectedColor
		} else if (isMoveTarget && isHovering) {
			return hoveringColor
		} else {
			return 'transparent';
		}
	}

	function getFileMarker(file: number) {
		return String.fromCharCode('a'.charCodeAt(0)
			+ (isWhitePerspective
				? file
				: (boardState.file.length - 1) - file))
	}
</script>

<div class="board-wrapper">
	<div bind:this={boardElement} class="board" style:width="{11 * hexHeight}px;" style:height="{11.67 * hexHeight}px;">
		{#each boardPieces as [index, p] (index)}
			{@const { file, rank } = p}
			{@const top = getTop(file, rank, isWhitePerspective)}
			{@const left = getLeft(file, isWhitePerspective)}
			{@const isMoveTarget = pmMap.get(makeMoveKey(file, rank))}
			{@const isSelected = hexEq(selected, p)}
			{@const isPromoting = hexEq(promotion?.to, p)}
			{@const isDraggable = draggable !== "none" && (draggable === "anyone" || (draggable === "turn"))}
			<div class="piece-wrapper">
				<Piece
					isSelected={isSelected}
					isTransparent
					isAnnotatable
					isDraggable={isDraggable}
					isPromoting={isPromoting}
					initialLeft={left}
					initialTop={top}
					piece={p.piece}
					onSelectHexagon={isMoveTarget ? () => onClickHexagon(file, rank) : undefined}
					onSelectPiece={() => onSelectPiece?.({ file, rank })}
					onDeSelectPiece={() => onDeSelectPiece?.({ file, rank })}
					onDragPiece={onDragBoardPiece}
					onDropPiece={(x, y) => onDropBoardPiece({ file, rank }, x, y, p.piece)}
					onCompletePromotion={onCompletePromotion}
				/>
			</div>
		{/each}
		{#each boardState.file as piecesFile, file}
			{@const fileMarker = getFileMarker(file)}
			{#each piecesFile.pieces as piece, rank}
				{@const hex = {file, rank}}
				{@const prevMoveFrom = {file: prevMove?.fromFile, rank: prevMove?.fromRank}}
				{@const prevMoveTo = {file: prevMove?.toFile, rank: prevMove?.toRank}}
				{@const rankMarker = rank + 1}
				{@const top = getTop(file, rank, isWhitePerspective)}
				{@const left = getLeft(file, isWhitePerspective)}
				{@const bgIndex = (colorsOffset[file] + rank) % 3}
				{@const isPrevMove = hexEq(prevMoveFrom, hex) || hexEq(prevMoveTo, hex)}
				{@const isMoveTarget = pmMap.get(makeMoveKey(file, rank))}
				{@const isSelected = hexEq(selected, hex)}
				{@const isHovering = hexEq(hovering, hex)}
				{@const bgColor = pickTileBgColor({isPrevMove, isSelected, isMoveTarget, isHovering})}
				{#if file === 0 || (file <= 5 && rank === ranksPerFile[file]-1)}
					<div
						class="hexagon-label"
						style:top="{top + (isWhitePerspective ? -hexHeight / 4 : hexHeight / 1.6)}px"
						style:left="{getLeft(file, true) - hexWidth / 4.5}px"
					>
						{rankMarker}
					</div>
				{/if}
				{#if file === boardState.file.length-1 || (file >= 5 && rank === ranksPerFile[file]-1)}
					<div
						class="hexagon-label"
						style:top="{top + (isWhitePerspective ? -hexHeight / 4 : hexHeight / 1.6)}px"
						style:left="{getLeft(file, true) + hexWidth}px"
					>
						{rankMarker}
					</div>
				{/if}
				<div
					class="hexagon"
					role="cell"
					tabindex="0"
					style:top="{top}px"
					style:left="{left}px"
					style:width="{hexWidth}px"
					style:height="{hexHeight}px"
					style:background-color={colors[bgIndex]}
				></div>
				<div
					class="hexagon"
					role="button"
					tabindex="0"
					style:top="{top}px"
					style:left="{left}px"
					style:width="{hexWidth}px"
					style:height="{hexHeight}px"
					style:background-color={bgColor}
					oncontextmenu={e => e.preventDefault()}
					onmousedown={() => onClickHexagon(file, rank)}
				>
					{#if piece !== pieces.empty}
						{#if isMoveTarget && !isHovering}
							<div
								class="move-circle"
								style:border-color={selectedColor}
								style:width="{hexWidth * 0.65}px"
								style:height="{hexWidth * 0.65}px"
							>
							</div>
						{/if}
					{:else}
						{#if isMoveTarget && !isHovering}
							<div class="move-dot" style:background-color={selectedColor}></div>
						{/if}
					{/if}
				</div>
			{/each}
			{@const top = isWhitePerspective ? getTop(file, -1, true) : getTop(file, -0.6, false)}
			{@const left = getLeft(file)}
			<div
				class="hexagon-label"
				style:top="{top}px"
				style:left="{left}px"
				style:width="{hexWidth}px"
				style:height="{hexHeight}px"
			>
				{fileMarker}
			</div>
		{/each}
	</div>
	{#await awaitingFen then fen}
		{#if fen}
			<Fen fen={fen}/>
		{/if}
	{/await}
</div>

<style>
    .board {
        position: relative;
		margin-top: 20px;
		margin-left: 20px;
    }

	.board-wrapper {
		margin-top: 20px;
	}

    .hexagon {
        cursor: pointer;
		z-index: 1;
        overflow: hidden;
        position: absolute;
        aspect-ratio: 1 / cos(30deg);
        clip-path: polygon(50% -50%, 100% 50%, 50% 150%, 0 50%);
        display: flex;
        justify-content: center;
        user-select: none;
        -moz-user-select: none;
        -webkit-user-select: none;
    }

	.hexagon-label {
		padding-top: 5px;
		text-align: center;
		position: absolute;
		font-weight: bold;
        z-index: 1;
		color: rgb(150,150,150);
        user-select: none;
        -moz-user-select: none;
        -webkit-user-select: none;
	}

    .move-dot {
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
		border-radius: 50%;
        height: 15px;
		width: 15px;
        cursor: pointer;
    }

    .move-circle {
        border: 5px solid;
        background-color: transparent;
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        border-radius: 50%;
        cursor: pointer;
    }
</style>

<script lang="ts">
	import { colors, colorsOffset, hexHeight, selectedColor, verticalFileOffsets, hexWidth, piecenames, defaultBoard, pieces } from '$lib/utils/globals';
	import type { ChessBoard } from '$lib/api/messages';
	import Piece from '$lib/components/chess/Piece.svelte';
	import type { Hexagon } from '$lib/api/model';
	import { isBlack, isWhite } from '$lib/utils/chess';

	export interface BoardProps {
		board?: ChessBoard;
		draggable?: "turn" | "anyone" | "none";
		isWhitePerspective: boolean;
		selectedHexagon?: Hexagon;
		potentialMoves?: Hexagon[];
		onSelectPiece?: (hex: Hexagon) => void;
		onDeSelectPiece?: (hex: Hexagon) => void;
	}

	const { board, draggable, isWhitePerspective, selectedHexagon, potentialMoves, onSelectPiece, onDeSelectPiece }: BoardProps = $props();

	function onSelectBoardPiece(hex: Hexagon) {
		onSelectPiece?.(hex);
	}

	function onDeSelectBoardPiece(hex: Hexagon) {
		onDeSelectPiece?.(hex);
	}

	const potentialMovesMap = $derived.by(() => {
		if (!potentialMoves) {
			return {};
		}
		const potentialMovesMap: Record<string, boolean> = {};
		for (const move of potentialMoves) {
			potentialMovesMap[move.file + "," + move.rank] = true;
		}
		return potentialMovesMap;
	});

	const actualBoard = $derived.by(() => board ? board : defaultBoard)
</script>

<div class="board" style="width: {11 * hexHeight}px; height: {11 * hexHeight}px;">
	{#each actualBoard.file as piecesFile, file (file)}
		{#each piecesFile.pieces as piece, rank (rank)}
			{@const top = rank * hexHeight + (verticalFileOffsets[file] * hexHeight) / 2}
			{@const flippedTop = 10 * hexHeight - top}
			{@const actualTop = isWhitePerspective ? flippedTop : top}
			{@const left = file * (hexHeight - 7)}
			{@const bgIndex = (colorsOffset[file] + rank) % 3}
			{@const isMove = potentialMovesMap[file + "," + rank]}
			{@const isSelected = selectedHexagon?.file === file && selectedHexagon?.rank === rank}
			{@const isDraggable =
				draggable !== "none" &&
				(draggable === "anyone" ||
				(draggable === "turn" && isWhite(piece) && board?.isWhiteTurn) ||
				(draggable === "turn" && isBlack(piece) && !board?.isWhiteTurn))}
			<div
				class="hexagon"
				role="cell"
				tabindex="0"
				style:top="{actualTop}px"
				style:left="{left}px"
				style:width="{hexWidth}px"
				style:height="{hexHeight}px"
				style:background-color={isSelected ? selectedColor : colors[bgIndex]}
				oncontextmenu={e => e.preventDefault()}
			>
				{#if piece !== pieces.empty}
					{#if isMove}
						<div class="move-circle"></div>
					{/if}
				{:else}
					{#if isMove}
						<div class="move-dot"></div>
					{/if}
				{/if}
			</div>
			{#if piece !== pieces.empty}
				<Piece
					{isSelected}
					isBgTransparent
					isDraggable={isDraggable}
					initialLeft={left}
					initialTop={actualTop}
					piece={piece}
					onSelectPiece={() => onSelectBoardPiece({ file, rank })}
					onDeSelectPiece={() => onDeSelectBoardPiece({ file, rank })}
				/>
			{/if}
		{/each}
	{/each}
</div>

<style>
    .board {
        position: relative;
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
        -khtml-user-select: none;
        -webkit-user-select: none;
    }

    .move-dot {
		background-color: rgba(100, 100, 100, 0.5);
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
        border: 5px solid rgba(100, 100, 100, 0.5);
        background-color: transparent;
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        border-radius: 50%;
        height: 50px;
        width: 50px;
        cursor: pointer;
    }
</style>

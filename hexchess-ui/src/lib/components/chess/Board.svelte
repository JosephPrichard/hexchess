<script lang="ts">
	import { colors, colorsOffset, height, piecenames, selectedColor, verticalFileOffsets, width } from '$lib/utils/globals';
	import type { ChessBoard, Hexagon } from '$lib/api/messages';
	import Piece from '$lib/components/chess/Piece.svelte';

	export interface BoardProps {
		board: ChessBoard;
		draggable?: "white" | "black";
		isWhitePerspective: boolean;
		selectedHexagon?: Hexagon;
		potentialMoves?: Hexagon[];
		onClickPiece?: (hex: Hexagon) => void;
	}

	const { board, draggable, isWhitePerspective, selectedHexagon, potentialMoves, onClickPiece }: BoardProps = $props();

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
</script>

<div class="board" style="width: {11 * height}px; height: {11 * height}px;">
	{#each board.file as piecesFile, file (file)}
		{#each piecesFile.pieces as piece, rank (rank)}
			{@const top = rank * height + (verticalFileOffsets[file] * height) / 2}
			{@const flippedTop = 10 * height - top}
			{@const left = file * (height - 8)}
			{@const bgIndex = (colorsOffset[file] + rank) % 3}
			{@const isMove = potentialMovesMap[file + "," + rank]}
			<div
				class="hexagon"
				style:top="{isWhitePerspective ? flippedTop : top}px"
				style:left="{left}px"
				style:width="{width}px"
				style:height="{height}px"
				style:background={selectedHexagon?.file === file && selectedHexagon?.rank === rank ? selectedColor : colors[bgIndex]}
				style:cursor={isMove ? "pointer" : undefined}
			>
				{#if piece !== 0}
					<Piece draggable={true} file={file} rank={rank} piece={piece} onClickPiece={onClickPiece}/>
					{#if isMove}
						<div class="move-circle"></div>
					{/if}
				{:else}
					{#if isMove}
						<div class="move-dot"></div>
					{/if}
				{/if}
			</div>
		{/each}
	{/each}
</div>

<style>
    .board {
        position: relative;
    }

    .hexagon {
        position: absolute;
        aspect-ratio: 1 / cos(30deg);
        clip-path: polygon(50% -50%, 100% 50%, 50% 150%, 0 50%);
        display: flex;
        justify-content: center;
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

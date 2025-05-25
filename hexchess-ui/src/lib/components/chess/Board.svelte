<script lang="ts">
	import { type ChessBoard, Piece, piecenames } from '$lib/models.js';

	export interface BoardProps {
		board: ChessBoard;
		isWhitePerspective: boolean;
	}

	const { board, isWhitePerspective }: BoardProps = $props();

	const height = 64;
	const width = height * 1.2;
	const verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5];
	const colors = ['rgb(255, 207, 159)', 'rgb(233, 172, 112)', 'rgb(210,140,69)'];
	const colorsOffset = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0];
</script>

<div class="board" style="width: {11 * height}px; height: {11 * height}px;">
	{#each board.pieces as piecesFile, file (file)}
		{#each piecesFile as piece, rank (rank)}
			{@const top = rank * height + (verticalFileOffsets[file] * height) / 2}
			{@const flippedTop = 10 * height - top}
			{@const left = file * (height - 8)}
			{@const bgIndex = (colorsOffset[file] + rank) % 3}
			{@const bgColor = colors[bgIndex]}
			<div
				class="hexagon"
				style:top="{isWhitePerspective ? flippedTop : top}px"
				style:left="{left}px"
				style:width="{width}px"
				style:height="{height}px"
				style:background={bgColor}
			>
				{#if piece !== Piece.empty}
					<img class="piece-img" src="/pieces/{piecenames[piece]}.png" alt="" draggable={false} />
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

    .piece-img {
        padding-top: 2px;
        max-width: 88%;
        max-height: 88%;
        cursor: pointer;
    }
</style>

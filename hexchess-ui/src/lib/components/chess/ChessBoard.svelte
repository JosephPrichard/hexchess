<script lang="ts">
    import { type ChessBoard, Piece, pieceNames } from '$lib/models.js';

    interface Props {
        board: ChessBoard;
        isBlackPerspective: boolean;
    }

    const { board, isBlackPerspective }: Props = $props();

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
            <div class="hexagon" style:top="{isBlackPerspective ? flippedTop : top}px" style:left="{left}px" style:width="{width}px" style:height="{height}px" style:background={bgColor}>
                {#if piece !== Piece.empty}
                    <img class="piece-img" src={`%sveltekit.assets%/pieces/${pieceNames[piece]}.png`} alt="" draggable={false} />
                {/if}
            </div>
        {/each}
    {/each}
</div>

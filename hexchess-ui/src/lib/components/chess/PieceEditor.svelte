<script lang="ts">
	import { hexHeight, hexWidth, piecenames } from '$lib/utils/globals';
	import Piece from '$lib/components/chess/Piece.svelte';

	export interface PieceEditorProps {
		pieces: number[]
	}

	const { pieces }: PieceEditorProps = $props();


</script>

<div class="piece-panel">
	{#each pieces as piece, i}
		<div
			class="piece-tile"
			style:width="{hexWidth}px"
			style:height="{hexHeight}px"
		>
			<Piece
				isDraggable
				initialLeft={0}
				initialTop={0}
				piece={piece}
			/>
		</div>
	{/each}
</div>

<style>
	.piece-panel {
		border-radius: 4px;
		background-color: rgb(55, 55, 55);
		flex: 0.5;
        box-shadow: inset 0 1px 4px rgba(0, 0, 0, 0.3);
		position: relative;
	}

	.piece-tile:hover {
        background-color: rgb(30, 144, 255, 0.2);
	}

    .piece-tile {
		position: relative;
		padding: 1px;
	}
</style>
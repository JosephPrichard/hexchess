<script lang="ts">
	import { getPiecePoints, makePieceWhite, piecenames } from '$lib/utils/chess.js';

	const { myPieces, theirPieces }: { myPieces?: number[]; theirPieces?: number[] } = $props();

	const pointsDiff = $derived.by(() => {
		const myPoints = myPieces?.reduce((acc, piece) => acc + getPiecePoints(piece), 0) || 0;
		const theirPoints = theirPieces?.reduce((acc, piece) => acc + getPiecePoints(piece), 0) || 0;
		return myPoints - theirPoints;
	});

	const pointsDiffStr = $derived(pointsDiff > 0 ? `+${pointsDiff}` : pointsDiff);
</script>

<div class="pieces-wrapper">
	{#each myPieces as piece}
		<div class="piece-icon-wrapper">
			<img class="piece-icon" src="/pieces/{piecenames[makePieceWhite(piece)]}.png" draggable={false} alt="" />
		</div>
	{/each}
	{#if pointsDiff > 0}
		<div class="points-diff">
			{pointsDiffStr}
		</div>
	{/if}
</div>

<style>
    .pieces-wrapper {
        width: 280px;
        display: flex;
        /*justify-content: center;*/
        flex-wrap: wrap;
    }

	.points-diff {
		font-size: 18px;
		margin-left: 25px;
		display: flex;
		align-items: center;
	}

    .piece-icon {
        width: 35px;
        height: 35px;
        margin-right: -25px;
		opacity: 0.7;
    }

    .piece-icon-wrapper {
        display: inline-block;
    }
</style>
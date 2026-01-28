<script lang="ts">
	import { getPiecePoints, makePieceWhite, piecenames, pieces } from '$lib/service/chess';

	interface TakenListProps {
		myPieces?: number[];
		theirPieces?: number[];
		showZero?: boolean;
	}

	let { myPieces, theirPieces, showZero }: TakenListProps = $props();

	const totalPoints = (pieces?: number[]) => pieces?.reduce((acc, piece) => acc + getPiecePoints(piece), 0) ?? 0;

	function aggregatePieces(pieceList?: number[]) {
		pieceList = pieceList?.sort((a, b) => a - b) ?? [];

		const aggregates: number[][] = [];
		let aggList: number[] = [];
		for (const piece of pieceList) {
			if (aggList.length < 1 || aggList[0] != piece) {
				aggList = [piece];
				aggregates.push(aggList);
			} else {
				aggList.push(piece);
			}
		}

		return aggregates;
	}

	const pointsDiff = $derived.by(() => totalPoints(myPieces) - totalPoints(theirPieces));

	const pointsDiffStr = $derived(pointsDiff >= 0 ? `+${pointsDiff}` : pointsDiff);
</script>

<div class="pieces-list-wrapper">
	<div class="points-diff">
		{#if showZero || (myPieces?.length ?? 0) > 0}
			{pointsDiffStr}
		{/if}
	</div>
	{#each aggregatePieces(myPieces) || [] as pieces}
		<div class="pieces-wrapper">
			{#each pieces as piece}
				<div class="piece-icon-wrapper">
					<img class="piece-icon" src="/pieces/{piecenames[makePieceWhite(piece)]}.png" draggable={false} alt="" />
				</div>
			{/each}
		</div>
	{/each}
</div>

<style>
    .pieces-list-wrapper {
        height: 35px;
        display: flex;
        align-items: center;
        padding-right: 10px;
        overflow: hidden;
    }

    .pieces-wrapper {
        margin-right: 20px;
    }

    .points-diff {
        font-size: 14px;
        line-height: 10px;
    }

    .piece-icon {
        position: relative;
        top: 1px;
        width: 30px;
        height: 30px;
        margin-right: -25px;
        opacity: 0.75;
    }

    .piece-icon-wrapper {
        display: inline-block;
    }
</style>
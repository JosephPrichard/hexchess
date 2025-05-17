<script lang="ts">
	import type { Move } from '$lib/models';
	import { stringOfMove } from '$lib/chess';

	export interface MoveListProps {
		moveList: Move[];
		onSelectMove?: (i: number) => void;
		selectedMoveIndex?: number;
	}

	const { moveList, onSelectMove, selectedMoveIndex }: MoveListProps = $props();
</script>

<div class="move-list">
	{#each moveList as moveOne, i (i)}
		{#if i % 2 === 0}
			{@const moveTwoIndex = i + 1}
			{@const moveTwo = moveList[i + 1]}
			<div class="move-row">
				<div class="move-number">
					{i / 2 + 1}.
				</div>
				<button
					class="move-button"
					class:move-button-hover={onSelectMove !== undefined}
					class:selected-move={selectedMoveIndex === i}
					onclick={() => onSelectMove ? onSelectMove(i) : {}}
					tabindex="-1"
				>
					{stringOfMove(moveOne)}
				</button>
				{#if moveTwo}
					<button
						class="move-button"
						class:move-button-hover={onSelectMove !== undefined}
						class:selected-move={selectedMoveIndex === moveTwoIndex}
						onclick={() =>  onSelectMove ? onSelectMove(moveTwoIndex) : {}}
						tabindex="-1"
					>
						{stringOfMove(moveTwo)}
					</button>
				{:else}
					<div class="move"></div>
				{/if}
			</div>
		{/if}
	{/each}
</div>

<script lang="ts">
	import type { PieceMove } from '$lib/models';
	import { stringOfMove } from '$lib/chess';

	export interface MoveListProps {
		moveList: PieceMove[];
		onSelectMove?: (i: number) => void;
		selectedMoveIndex?: number;
	}

	const { moveList, onSelectMove, selectedMoveIndex }: MoveListProps = $props();
</script>

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

<style>
    .move-row {
        display: flex;
        flex-direction: row;
        height: 40px;
    }

    .move-number {
        flex: 0.2;
        line-height: 40px;
        text-align: center;
        background-color: rgb(48, 48, 48);
        border-right: 1px solid rgb(58, 58, 58);
        border-left: 1px solid rgb(58, 58, 58);
    }

    .move-button {
        all: unset;
        cursor: pointer;
        flex: 0.4;
        line-height: 40px;
        padding-left: 20px;
        border-radius: 2px;
        -moz-user-select: none;
        -khtml-user-select: none;
        -webkit-user-select: none;
    }

    .move-button-hover:hover {
        background-color: rgb(65, 65, 65);
    }

    .selected-move {
        background-color: rgb(53, 53, 53);
    }
</style>
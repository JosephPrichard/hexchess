<script lang="ts">
	import type { PieceMove } from '../../api/messages';
	import { stringOfMove } from '../../services/chess';

	export interface MoveListProps {
		moveList: PieceMove[];
		onSelectMove?: (i: number) => void;
		selectedMoveIndex?: number;
		completeMessage?: string;
	}

	const { moveList, onSelectMove, selectedMoveIndex, completeMessage }: MoveListProps = $props();
</script>

<div class="growing-scrollbox">
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
					onclick={() => onSelectMove?.(i)}
					tabindex="-1"
				>
					{stringOfMove(moveOne)}
				</button>
				{#if moveTwo}
					<button
						class="move-button"
						class:move-button-hover={onSelectMove !== undefined}
						class:selected-move={selectedMoveIndex === moveTwoIndex}
						onclick={() => onSelectMove?.(moveTwoIndex)}
						tabindex="-1"
					>
						{stringOfMove(moveTwo)}
					</button>
				{:else}
					<div class="move-button"></div>
				{/if}
			</div>
		{/if}
	{/each}
	{#if completeMessage}
		<div class="completed-message">
			{completeMessage}
		</div>
	{/if}
</div>

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
        border-radius: 1px;
        -moz-user-select: none;
        -khtml-user-select: none;
        -webkit-user-select: none;
        border: rgba(0, 0, 0, 0) solid 1px;
    }

    .move-button-hover:hover {
        border: rgb(65, 65, 65) solid 1px;
    }

    .selected-move {
        background-color: rgba(51, 153, 255, 0.25);
    }

    .completed-message {
        text-align: center;
        margin-top: 10px;
        margin-bottom: 10px;
        font-style: italic;
    }
</style>
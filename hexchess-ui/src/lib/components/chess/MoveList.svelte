<script lang="ts">
	import { moveElementHeight } from '$lib/components/chess/render';

	export interface MoveListProps {
		containerElement?: HTMLElement;
		moveList: string[];
		onSelectMove?: (i: number) => void;
		selectedMoveIndex?: number;
		completeMessage?: string;
	}

	let { containerElement = $bindable(), moveList, onSelectMove, selectedMoveIndex, completeMessage }: MoveListProps = $props();
</script>

<div class="growing-scrollbox moves" bind:this={containerElement}>
	<div class="move-numbers">
		{#each moveList as _, i (i)}
			{#if i % 2 === 0}
				<div class="move-row">
					<div class="move-number" style:line-height="{moveElementHeight}px" class:last-move-number={i === moveList.length-1}>
						{i / 2 + 1}.
					</div>
				</div>
			{/if}
		{/each}
	</div>
	<div class="move-pairs">
		{#each moveList as moveOne, i (i)}
			{#if i % 2 === 0}
				{@const moveTwoIndex = i + 1}
				{@const moveTwo = moveList[i + 1]}
				<div class="move-row">
					<button
						class="move-button"
						style:height="{moveElementHeight}px"
						class:move-button-hover={onSelectMove !== undefined}
						class:selected-move={selectedMoveIndex === i}
						onclick={() => onSelectMove?.(i)}
						tabindex="-1"
					>
						{moveOne}
					</button>
					{#if moveTwo}
						<button
							class="move-button"
							style:height="{moveElementHeight}px"
							class:move-button-hover={onSelectMove !== undefined}
							class:selected-move={selectedMoveIndex === moveTwoIndex}
							onclick={() => onSelectMove?.(moveTwoIndex)}
							tabindex="-1"
						>
							{moveTwo}
						</button>
					{:else}
						<div class="move-button"></div>
					{/if}
				</div>
			{/if}
		{/each}
	</div>
	{#if completeMessage}
		<div class="completed-message">
			{completeMessage}
		</div>
	{/if}
</div>

<style>
	.moves {
		display: flex;
		flex-direction: row;
	}

    .move-row {
        display: flex;
        flex-direction: row;
        height: 35px;
    }

	.move-pairs {
		flex: 0.8;
		display: flex;
		flex-direction: column;
	}

    .move-numbers {
		flex: 0.2;
		height: fit-content;
        /*box-shadow: 4px 4px 4px -4px rgba(0, 0, 0, 0.6);*/
	}

	.last-move-number {
        /*border-bottom-right-radius: 3px;*/
	}

    .move-number {
        width: 100%;
        /*line-height: 35px;*/
        text-align: center;
        background-color: rgb(48, 48, 48);
    }

    .move-button {
        all: unset;
        cursor: pointer;
        flex: 0.5;
        /*line-height: 35px;*/
        padding-left: 20px;
        -moz-user-select: none;
        -khtml-user-select: none;
        -webkit-user-select: none;
    }

    .move-button-hover:hover {
        background-color: rgba(51, 153, 255, 0.25);
    }

    .selected-move {
        background-color: rgba(51, 153, 255, 0.75) !important;
    }

    .completed-message {
        text-align: center;
        margin-top: 10px;
        margin-bottom: 10px;
        font-style: italic;
    }
</style>
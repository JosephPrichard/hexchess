<script lang="ts">
    import { stringOfMove } from '$lib/utils.js';
    import type { Move } from '$lib/models';

    interface Props {
        moveList: Move[];
        onSelectMove: (i: number) => void;
        selectedMoveIndex?: number;
    }

    const { moveList, onSelectMove, selectedMoveIndex }: Props = $props();
</script>

<div class="move-list">
    {#each moveList as moveOne, i (i)}
        {#if i % 2 === 0}
            {@const moveTwoIndex = i + 1}
            {@const moveTwo = moveList[i + 1]}
            <div class="move-row">
                <div class="move-number">{i / 2 + 1}.</div>
                <div class="move" class:selected-move={selectedMoveIndex === i} role="button" onkeydown={() => onSelectMove(i)} tabindex="-1">
                    {stringOfMove(moveOne)}
                </div>
                {#if moveTwo}
                    <div class="move" class:selected-move={selectedMoveIndex === moveTwoIndex} role="button" onkeydown={() => onSelectMove(moveTwoIndex)} tabindex="-1">
                        {stringOfMove(moveTwo)}
                    </div>
                {:else}
                    <div class="move"></div>
                {/if}
            </div>
        {/if}
    {/each}
</div>

<script lang="ts">
    import { stringOfMove } from '$lib/utils.js';
    import type { Move } from '$lib/models';

    interface Props {
        moveList: Move[];
        onClickMove: (i: number, move: Move) => void;
    }

    const { moveList, onClickMove }: Props = $props();
</script>

<div class="move-list">
    {#each moveList as moveOne, i (i)}
        {#if i % 2 === 0}
            {@const moveTwo = moveList[i + 1]}
            <div class="move-row">
                <div class="move-number">{i / 2 + 1}.</div>
                <div class="move" role="button" onkeydown={() => onClickMove(i, moveList[i])} tabindex="-1">
                    {stringOfMove(moveOne)}
                </div>
                {#if moveTwo}
                    <div class="move" role="button" onkeydown={() => onClickMove(i + 1, moveTwo)} tabindex="-1">
                        {stringOfMove(moveTwo)}
                    </div>
                {:else}
                    <div class="move"></div>
                {/if}
            </div>
        {/if}
    {/each}
</div>

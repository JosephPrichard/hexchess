<script lang="ts">
	import { piecenames, type PieceType } from '$lib/models.js';

	const { pieces }: { pieces: PieceType[] } = $props();
</script>

<div class="pieces-wrapper">
	{#each pieces as piece}
		<div class="piece-icon-wrapper">
			<img class="piece-icon" src="/pieces/{piecenames[piece]}.png" draggable={false} alt="" />
		</div>
	{/each}
</div>

<style>
    .pieces-wrapper {
        width: 280px;
        display: flex;
        justify-content: center;
        flex-wrap: wrap;
    }

    .piece-icon {
        width: 35px;
        height: 35px;
        margin-right: -20px;
    }

    .piece-icon-wrapper {
        display: inline-block;
    }
</style>
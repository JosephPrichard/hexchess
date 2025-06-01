<script lang="ts">
	import type { PlayerView } from '$lib/models.js';

	const { player, isTurn }: { player: PlayerView | undefined, isTurn: boolean } = $props();
</script>

<div class="side-table-header player-panel">
	{#if player}
		<div class="side-table-header-elem text-xsm">
			<div class="turn-circle" class:turn-circle-green={isTurn}></div>
			<a href={player.isGuest ? undefined : `/players/${player.id}`} class="text-ul">
				<b>{player.name}</b>
			</a>
			<img class="flag-md" src="/flags/{player.country}.png" alt="" />
			{#if player.elo}
				<span>({player.elo})</span>
			{/if}
		</div>
	{/if}
</div>

<style>
    .player-panel {
        padding-top: 15px;
        padding-bottom: 15px;
    }

    .turn-circle {
        display: inline-block;
        width: 7px;
        height: 7px;
        margin-right: 5px;
        border-radius: 50%;
        border: 2px solid #B4B4B4;
        background-color: transparent;
    }

    .turn-circle-green {
        border-color: #78b13f;
        background-color: #78b13f;
    }
</style>
<script lang="ts">
	import type { PlayerModel } from '$lib/api/model';
	import type { Player } from '$lib/api/messages';

	const { player, isTurn }: { player: PlayerModel | Player | undefined, isTurn: boolean } = $props();
</script>


{#if player}
	<div class="side-table-header-elem text-xsm">
		<div class="turn-circle" class:turn-circle-green={isTurn}></div>
		{#if !player.isGuest}
			<a href="/players/{player.id}" class="text-ul">
				<b>{player.name}</b>
			</a>
		{:else}
			<span class="text-ul">
				<b>{player.name}</b>
			</span>
		{/if}
		<img class="flag-md" src="/flags/{player.country}.png" alt="" />
		{#if player.elo}
			<span>({player.elo})</span>
		{/if}
	</div>
{/if}

<style>
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
<script lang="ts">
	import type { PlayerModel } from '$lib/api/model';
	import type { PlayerState } from '$lib/api/messages';

	export interface PlayerPanelProps {
		player: PlayerModel | PlayerState | undefined,
		isTurn: boolean
	}

	const { player, isTurn }: PlayerPanelProps = $props();
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
</style>
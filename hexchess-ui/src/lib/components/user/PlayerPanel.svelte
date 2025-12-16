<script lang="ts">
	import type { PlayerModel } from '$lib/api/models';
	import type { PlayerState } from '$lib/pb/messages';

	export interface PlayerPanelProps {
		player: PlayerModel | PlayerState | undefined,
		self: PlayerModel | PlayerState | undefined,
		isTurn: boolean
	}

	const { player, self, isTurn }: PlayerPanelProps = $props();
</script>

{#if player}
	<div class="side-table-header-elem text-xsm" class:self-color={self !== undefined && player?.id === self?.id}>
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
{:else}
	<div class="side-table-header-elem text-xsm">
		<div class="turn-circle" class:turn-circle-green={isTurn}></div>
		<span class="waiting-text">
			Waiting for player...
		</span>
	</div>
{/if}

<style>
	.self-color {
		color: dodgerblue;
	}

	.waiting-text {
		color: rgb(150, 150, 150);
	}
</style>
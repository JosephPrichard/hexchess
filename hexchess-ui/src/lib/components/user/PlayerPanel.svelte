<script lang="ts">
	import type { PlayerModel } from '$lib/api/models';
	import type { PlayerState } from '$lib/pb/messages';

	export interface PlayerPanelProps {
		player: PlayerModel | PlayerState | undefined,
		self: PlayerModel | PlayerState | undefined,
		isTurn: boolean
	}

	const { player, self, isTurn }: PlayerPanelProps = $props();

	const playerName = $derived(player?.name !== "" ? player?.name : "-");
	const isMe = $derived(self !== undefined && player?.id === self?.id);
</script>

{#if player}
	<div class="side-table-header-elem text-xsm" class:self-color={isMe}>
		<div class="turn-circle" class:turn-circle-green={isTurn}></div>
		{#if !player.isGuest}
			<a href="/players/{player.id}" class="text-ul">
				<b>{playerName}</b>
			</a>
		{:else}
			<span class="text-ul">
				<b>{playerName}</b>
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
	.waiting-text {
		color: rgb(150, 150, 150);
	}
</style>
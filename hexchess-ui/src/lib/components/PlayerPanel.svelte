<script lang="ts">
	import type { PlayerModel } from '$lib/api/models';
	import type { PlayerState } from '$lib/pb/messages';
	import ProfilePic from '$lib/components/ProfilePic.svelte';

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
	<div class="side-table-header-elem text-xsm player-panel-wrapper" class:self-color={isMe}>
		<div class="player-element">
			<div class="turn-circle" class:turn-circle-green={isTurn}></div>
		</div>
		<ProfilePic userId={Number(player.id)} size={35}/>
		<div class="player-element player-name">
			{#if !player.isGuest}
				<a href="/players/{player.id}" class="text-ul">
					<b>{playerName}</b>
				</a>
			{:else}
				<span>
					<b>{playerName}</b>
				</span>
			{/if}
			{#if player.elo}
				<span>({player.elo})</span>
			{/if}
		</div>
	</div>
{:else}
	<div class="side-table-header-elem text-xsm player-panel-wrapper">
		<div class="turn-circle-wrapper">
			<div class="turn-circle" class:turn-circle-green={isTurn}></div>
		</div>
		<span class="waiting-text">
			Waiting for player...
		</span>
	</div>
{/if}

<style>
	.player-name {
		width: 275px;
	}

	.player-element {
		display: flex;
		align-items: center;
        white-space: nowrap;
        overflow: hidden;
	}

	.player-panel-wrapper {
		display: flex;
		gap: 8px;
	}

	.waiting-text {
		color: rgb(150, 150, 150);
	}
</style>
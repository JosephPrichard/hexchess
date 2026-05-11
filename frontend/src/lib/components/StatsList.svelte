<script lang="ts">
	import { goto } from '$app/navigation';
	import type { LbdUserModel } from '$lib/api/models';
	import { MediaQuery } from 'svelte/reactivity';

	interface Props {
		userList: LbdUserModel[];
	}

	const { userList }: Props = $props();

	const isLarge = new MediaQuery('min-width: 768px');
</script>

{#if userList.length}
	<table class="table-container">
		<thead>
			<tr>
				{#if isLarge.current}
					<th style="width: 8%;">Rank</th>
					<th style="width: 40%;">Player</th>
					<th style="width: 12%;">Elo</th>
					<th style="width: 8%;">Win%</th>
					<th style="width: 8%;">Won</th>
					<th style="width: 8%;">Lost</th>
					<th style="width: 8%;">Drawn</th>
					<th style="width: 8%;">Total</th>
				{:else}
					<th style="width: 15%;">Rank</th>
					<th style="width: 40%;">Player</th>
					<th style="width: 15%;">Elo</th>
					<th style="width: 15%;">Win%</th>
					<th style="width: 15%;">Total</th>
				{/if}
			</tr>
		</thead>
		<tbody>
			{#each userList as user (user.id)}
				{@const wrClass = function() {
					if (user.winrate > 50) {
						return 'green-color';
					} else if (user.winrate < 50) {
						return 'red-color';
					} else {
						return 'yellow-color';
					}
				}()}
				{@const total = user.wins+user.losses+user.draws}
				{@const country = `/flags/${user.country}.png`}
				<tr class="row-hover" onclick={() => goto(`/players/${user.id}`)}>
					{#if isLarge.current}
						<td>{user.rank}</td>
						<td>
							{user.username}
							<img class="flag" src="{country}" alt="" />
						</td>
						<td>{Math.round(user.elo)}</td>
						<td class={wrClass}>
							{user.winrate}%
						</td>
						<td class="green-color">
							{user.wins}
						</td>
						<td class="red-color">
							{user.losses}
						</td>
						<td class="yellow-color">
							{user.draws}
						</td>
						<td>{total}</td>
					{:else}
						<td>{user.rank}</td>
						<td>
							{user.username}
						</td>
						<td>{Math.round(user.elo)}</td>
						<td class={wrClass}>
							{user.winrate}%
						</td>
						<td>{total}</td>
					{/if}
				</tr>
			{/each}
		</tbody>
	</table>
{:else}
	<div class="color-wrapper">No more players to show</div>
{/if}
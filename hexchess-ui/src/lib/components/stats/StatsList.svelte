<script lang="ts">
	import { goto } from '$app/navigation';
	import { getWinrateClass } from '$lib/api/models';
	import type { UserModel } from '../../api/models';

	interface Props {
		userList: UserModel[];
	}

	const { userList }: Props = $props();
</script>

{#if userList.length}
	<table class="table-container">
		<thead>
			<tr>
				<th style="width: 9%">Rank</th>
				<th style="width: 43%">Player</th>
				<th style="width: 12%">Elo</th>
				<th style="width: 9%">Win%</th>
				<th style="width: 9%">Won</th>
				<th style="width: 9%">Lost</th>
				<th style="width: 9%">Total</th>
			</tr>
		</thead>
		<tbody>
			{#each userList as user (user.id)}
				{@const wrClass = function() {
					if (user.winRate > 50) {
						return 'green-color';
					} else if (user.winRate < 50) {
						return 'red-color';
					} else {
						return 'yellow-color';
					}
				}()}
				<tr class="row-hover" onclick={() => goto(`/players/${user.id}`)}>
					<td>{user.rank}</td>
					<td>
						{user.username}
						<img class="flag" src="/flags/{user.country}.png" alt="" />
					</td>
					<td >{user.elo}</td>
					<td class={wrClass}>
						{user.winRate}%
					</td>
					<td class="green-color">
						{user.wins}
					</td>
					<td class="red-color">
						{user.losses}
					</td>
					<td>{user.total}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{:else}
	<div class="color-wrapper">No more players to show</div>
{/if}

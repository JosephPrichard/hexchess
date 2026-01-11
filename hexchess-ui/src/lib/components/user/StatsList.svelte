<script lang="ts">
	import { goto } from '$app/navigation';
	import type { LbdUserModel } from '$lib/api/models';
	import ProfilePic from '$lib/components/user/ProfilePic.svelte';

	interface Props {
		userList: LbdUserModel[];
	}

	const { userList }: Props = $props();
</script>

{#if userList.length}
	<table class="table-container">
		<thead>
			<tr>
				<th style="width: 8%;">Rank</th>
				<th style="width: 40%;">Player</th>
				<th style="width: 12%;">Elo</th>
				<th style="width: 8%;">Win%</th>
				<th style="width: 8%;">Won</th>
				<th style="width: 8%;">Lost</th>
				<th style="width: 8%;">Drawn</th>
				<th style="width: 8%;">Total</th>
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
				<tr class="row-hover" onclick={() => goto(`/players/${user.id}`)}>
					<td>{user.rank}</td>
					<td>
<!--						<ProfilePic userId={user.id} size={20}/>-->
						{user.username}
						<img class="flag" src="/flags/{user.country}.png" alt="" />
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
					<td>{user.wins+user.losses+user.draws}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{:else}
	<div class="color-wrapper">No more players to show</div>
{/if}

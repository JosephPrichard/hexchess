<script lang="ts">
    import { goto } from '$app/navigation';
    import type { UserView } from '$lib/models';

    interface Props {
        userList: UserView[];
    }

    const { userList }: Props = $props();
</script>

{#if userList.length}
    <table id="stats-table" class="table-container">
        <thead>
            <tr>
                <th>Rank</th>
                <th>Player</th>
                <th>Elo</th>
                <th>Win%</th>
                <th>Won</th>
                <th>Lost</th>
                <th>Total</th>
            </tr>
        </thead>
        <tbody>
            {#each userList as user (user.id)}
                <tr id={'user-' + user.id} class="row-hover" onclick={() => goto(`/players/${user.id}`)}>
                    <td style="width: 9%">{user.rank}</td>
                    <td style="width: 43%">
                        {user.username}
                        <img class="flag" src={`%sveltekit.assets%/flags/${user.country}.png`} alt="" />
                    </td>
                    <td style="width: 12%">{user.elo}</td>
                    <td class={user.winRateColor} style="width: 9%">
                        {user.winRate}%
                    </td>
                    <td class="green-color" style="width: 9%">{user.wins}</td>
                    <td class="red-color" style="width: 9%">{user.losses}</td>
                    <td style="width: 9%">{user.total}</td>
                </tr>
            {/each}
        </tbody>
    </table>
{:else}
    <div class="color-wrapper">No players to show</div>
{/if}

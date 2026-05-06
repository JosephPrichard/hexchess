<script lang="ts">
    import {GameModeNameMap, type UserStatsEntity} from "$lib/api/models";
    import {MediaQuery} from "svelte/reactivity";

    const { userStats }: { userStats: UserStatsEntity } = $props();

    const isLarge = new MediaQuery('min-width: 768px');
</script>

{#if (userStats.modeStats ?? []).length > 0}
    <table class="table-container">
        <thead>
        <tr>
            {#if isLarge.current}
                <th style="width: 20%;">Mode</th>
                <th style="width: 8%;">#Rank</th>
                <th style="width: 11%;">Elo</th>
                <th style="width: 11%;">Peak Elo</th>
                <th style="width: 10%;">Win%</th>
                <th style="width: 10%;">Won</th>
                <th style="width: 10%;">Lost</th>
                <th style="width: 10%;">Drawn</th>
                <th style="width: 10%;">Total</th>
            {:else}
                <th style="width: 36%;">Mode</th>
                <th style="width: 16%;">#Rank</th>
                <th style="width: 16%;">Elo</th>
                <th style="width: 16%;">Win%</th>
                <th style="width: 16%;">Total</th>
            {/if}
        </tr>
        </thead>
        <tbody>
        {#each userStats.modeStats as stats (stats.mode)}
            {@const wrClass = function() {
                if (stats.winrate > 50) {
                    return 'green-color';
                } else if (stats.winrate < 50) {
                    return 'red-color';
                } else {
                    return 'yellow-color';
                }
            }()}
            {@const gameModeName = GameModeNameMap[stats.mode] || "Unknown"}
            {@const total = stats.wins+stats.losses+stats.draws}
            {@const elo = Math.round(stats.elo)}
            {@const highestElo = Math.round(stats.highestElo)}
            {@const winrate = `${stats.winrate}%`}
            <tr>
                {#if isLarge.current}
                    <td>{gameModeName}</td>
                    <td>{stats.rank}</td>
                    <td>{elo}</td>
                    <td>{highestElo}</td>
                    <td class={wrClass}>
                        {winrate}
                    </td>
                    <td class="green-color">
                        {stats.wins}
                    </td>
                    <td class="red-color">
                        {stats.losses}
                    </td>
                    <td class="yellow-color">
                        {stats.draws}
                    </td>
                    <td>{total}</td>
                {:else}
                    <td>{gameModeName}</td>
                    <td>{stats.rank}</td>
                    <td>{elo}</td>
                    <td class={wrClass}>
                        {winrate}
                    </td>
                    <td>{total}</td>
                {/if}
            </tr>
        {/each}
        </tbody>
    </table>
{:else}
    <div class="color-wrapper">This player doesn't have any recorded stats yet.</div>
{/if}
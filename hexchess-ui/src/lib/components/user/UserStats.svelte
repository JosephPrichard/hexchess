<script lang="ts">
    import {GameModeNameMap, type UserStatsEntity} from "$lib/api/models";

    const { userStats }: { userStats: UserStatsEntity } = $props();
</script>

{#if (userStats.modeStats ?? []).length > 0}
    <table class="table-container">
        <thead>
        <tr>
            <th style="width: 20%;">Mode</th>
            <th style="width: 8%;">#Rank</th>
            <th style="width: 11%;">Elo</th>
            <th style="width: 11%;">Peak Elo</th>
            <th style="width: 10%;">Win%</th>
            <th style="width: 10%;">Won</th>
            <th style="width: 10%;">Lost</th>
            <th style="width: 10%;">Drawn</th>
            <th style="width: 10%;">Total</th>
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
            <tr>
                <td>{GameModeNameMap[stats.mode] || "Unknown"}</td>
                <td>{stats.rank}</td>
                <td>{Math.round(stats.elo)}</td>
                <td>{Math.round(stats.highestElo)}</td>
                <td class={wrClass}>
                    {stats.winrate}%
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
                <td>{stats.wins+stats.losses+stats.draws}</td>
            </tr>
        {/each}
        </tbody>
    </table>
{:else}
    <div class="color-wrapper">This player doesn't have any recorded stats yet.</div>
{/if}
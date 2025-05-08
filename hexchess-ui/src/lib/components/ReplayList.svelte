<script lang="ts">
    import { goto } from '$app/navigation';
    import type { ReplayView } from '$lib/models';

    interface Props {
        replayList: ReplayView[];
    }

    const { replayList }: Props = $props();
</script>

{#each replayList as replay (replay.id)}
    <tr id={'replay-' + replay.id} data-id={replay.id} class="row-hover" onclick={() => goto(`/games/replay/${replay.id}`)}>
        <td style="width: 25%">
            <a href={`/players/${replay.whiteId}`} class="text-ul">{replay.whiteName}</a>
            <img class="flag" src={`%sveltekit.assets%/flags/flags/${replay.whiteCountry}.png`} alt="" />
            <span class={replay.whiteEloColor}>
                {replay.whiteEloDiff}
            </span>
        </td>
        <td style="width: 25%">
            <a href={`/players/${replay.blackId}`} class="text-ul">{replay.blackName}</a>
            <img class="flag" src={`%sveltekit.assets%/flags/${replay.blackCountry}.png`} alt="" />
            <span class={replay.blackEloColor}>
                {replay.blackEloDiff}
            </span>
        </td>
        <td style="width: 25%">
            {replay.result}
        </td>
        <td style="width: 25%">
            {replay.playedOn}
        </td>
    </tr>
{/each}

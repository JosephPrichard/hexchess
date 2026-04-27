<script lang="ts">
    import {formatEloDiff, formatRelativeTime, getReplayColors, formatReplayResult} from "$lib/utils/format.js";
    import {GameModeNameMap, type ReplayModel} from "$lib/api/models.js";
    import {goto} from "$app/navigation";
    import ChallengeIcon from "$lib/components/icons/ChallengeIcon.svelte";

    const { replay, index }: { replay: ReplayModel, index: number } = $props();

    const [whiteClass, blackClass] = $derived(getReplayColors(replay.result));
    const reroute = () => goto(`/replay/${replay.id}`)
</script>

<div role="button"
     tabindex="0"
     class="replay-snippet-wrapper row-hover"
     class:replay-snippet-wrapper-alt={index % 2 === 0}
     onclick={reroute}
     onkeydown={reroute}
     aria-label="View replay"
>
    <div class="replay-snippet-mode">
        <b>{GameModeNameMap[replay.mode]}</b>
    </div>
    <div class="replay-snippet-playedon">
        {formatRelativeTime(replay.playedOn)}
    </div>
    <div class="replay-snippet-players">
        <div class="replay-snippet-player replay-snippet-players-left">
            <div>
                <a href="/players/{replay.whiteId}" class="text-ul player-name">
                    <b>{replay.whiteName}</b>
                    <img class="flag" src="/flags/{replay.blackCountry}.png" alt="" />
                </a>
            </div>
            <div>
                {Math.round(replay.whiteElo)}
                <span class={whiteClass}>{formatEloDiff(replay.whiteEloDiff)}</span>
            </div>
        </div>
        <ChallengeIcon/>
        <div class="replay-snippet-player">
            <div>
                <a href="/players/{replay.blackId}" class="text-ul player-name">
                   <b>{replay.blackName}</b>
                   <img class="flag" src="/flags/{replay.whiteCountry}.png" alt="" />
                </a>
            </div>
            <div>
                {Math.round(replay.blackElo)}
                <span class={blackClass}>{formatEloDiff(replay.blackEloDiff)}</span>
            </div>
        </div>
    </div>
    <div class="replay-snippet-result">
        {formatReplayResult(replay.result)}
    </div>
</div>

<style>
    .player-name {
        font-size: 19px;
    }

    .replay-snippet-wrapper {
        display: flex;
        flex-direction: column;
        gap: 5px;
        padding: 25px;
        background-color: rgb(38, 38, 38);
    }

    .replay-snippet-wrapper-alt {
        background-color: rgb(50, 50, 50);
    }

    .replay-snippet-wrapper:hover {
        background-color: rgba(43, 71, 94, 0.5);
    }

    .replay-snippet-mode {
        font-size: 20px;
    }

    .replay-snippet-playedon {
        font-size: 14px;
        color: rgb(160, 160, 160);
    }

    .replay-snippet-players {
        width: 100%;
        justify-content: center;

        gap: 10px;
        flex-direction: row;

        display: grid;
        grid-template-columns: 1fr auto 1fr;
        align-items: center;
    }

    .replay-snippet-players-left {
       text-align: right;
    }

    .replay-snippet-player {
        display: flex;
        flex-direction: column;
    }

    .replay-snippet-result {
        width: 100%;
        text-align: center;
    }
</style>
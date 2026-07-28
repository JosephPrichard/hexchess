<script lang="ts">
    import {formatEloDiff, formatRelativeTime, getReplayColors, formatReplayResult, formatReplayCause} from "$lib/utils/format.js";
    import {UntypedGameModeNameMap, type Replay} from "$lib/api/models.js";
    import {goto} from "$app/navigation";
    import {piecenames, pieces} from "$lib/service/chess";
    import ChallengeIcon from "$lib/icons/ChallengeIcon.svelte";

    type Rounding = "rounded-top" | "rounded-bottom" | undefined;

    const { replay, rounding, index }: { replay: Replay, rounding?: Rounding, index: number } = $props();

    const [whiteClass, blackClass] = $derived(getReplayColors(replay.result));
    const reroute = () => goto(`/replay/${replay.id}`);
</script>

<div role="button"
     tabindex="0"
     class="replay-snippet-root row-hover"
     onclick={reroute}
     onkeydown={reroute}
     aria-label="View replay"
     class:replay-snippet-root-alt={index % 2 === 0}
     class:rounded-top={rounding === "rounded-top"}
     class:rounded-bottom={rounding === "rounded-bottom"}
>
    <div class="replay-snippet-wrapper">
        <div class="replay-snippet-mode">
            <b>{UntypedGameModeNameMap[replay.mode]}</b>
        </div>
        <div class="replay-snippet-playedon">
            {formatRelativeTime(replay.playedOn)} • {replay.turnCount ?? 0} turns • {Math.round(replay.rating ?? 0)} rating
        </div>
        <div class="replay-snippet-bottom">
            <div class="replay-snippet-players">
                <div class="replay-snippet-player replay-snippet-players-left">
                    <div>
                        <a href="/players/{replay.whiteId}" class="text-ul player-name">
                            <img class="piece-icon" src="/pieces/{piecenames[pieces.whiteQueen]}.png" draggable={false} alt="" />
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
                            <img class="flag" src="/flags/{replay.whiteCountry}.png" alt="" />
                            <b>{replay.blackName}</b>
                            <img class="piece-icon" src="/pieces/{piecenames[pieces.blackQueen]}.png" draggable={false} alt="" />
                        </a>
                    </div>
                    <div>
                        {Math.round(replay.blackElo)}
                        <span class={blackClass}>{formatEloDiff(replay.blackEloDiff)}</span>
                    </div>
                </div>
            </div>
            <div class="replay-snippet-result">
                {formatReplayCause(replay.cause)} • {formatReplayResult(replay.result)}
            </div>
        </div>
    </div>
</div>

<style>
    .replay-snippet-root {
        padding: 15px;
        background-color: rgb(38, 38, 38);

        display: flex;
        flex-direction: row;
    }

    .replay-snippet-root-alt {
        background-color: rgb(50, 50, 50);
    }

    .replay-snippet-root:hover {
        background-color: rgba(43, 71, 94, 0.5);
    }

    .replay-snippet-wrapper {
        display: flex;
        flex-direction: column;
        gap: 5px;

        width: 100%;
    }

    .replay-snippet-bottom {
        height: 100%;
        vertical-align: middle;
    }

    .replay-snippet-mode {
        font-size: 17px;
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

        margin-bottom: 15px;
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
        font-size: 16px;
        margin-top: 10px;
        margin-bottom: 10px;
    }

    .piece-icon {
        width: 30px;
        height: 30px;
        position: relative;
        top: 7px;
    }

    .player-name {
        font-size: 17px;
    }

    .rounded-top {
        border-top-left-radius: 2px;
        border-top-right-radius: 2px;
    }

    .rounded-bottom {
        border-bottom-left-radius: 2px;
        border-bottom-right-radius: 2px;
    }
</style>
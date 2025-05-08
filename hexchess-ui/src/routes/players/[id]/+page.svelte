<script lang="ts">
    import type { ColorSelect, TimeControl, UserWithReplaysView } from '$lib/models.js';
    import { goto } from '$app/navigation';
    import CreateGame from '$lib/components/modals/CreateGame.svelte';
    import { getReplays, postCreateChallenge } from '$lib/api';
    import { onMount } from 'svelte';
    import { getClientSession } from '$lib/utils';
    import ChallengeSvg from '$lib/components/icons/ChallengeIcon.svelte';

    export interface PlayerProps {
        userWithReplays: UserWithReplaysView;
    }

    const { data }: { data: PlayerProps } = $props();
    const { user, replayList } = data.userWithReplays;

    let nestedReplayList = $state([replayList]);
    let showCreateModal = $state(false);
    let hasMoreReplays = $state(true);
    let isDifferentUser = $state(false);

    async function tryLoadReplays() {
        const lastId = nestedReplayList.at(-1)?.at(-1)?.id;
        const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight;

        if (hasMoreReplays && isAtPageBottom && lastId !== undefined && user?.id !== undefined) {
            const { ok, resp, err } = await getReplays(user.id, lastId);
            if (!ok) {
                console.error(ok, resp, err);
            }

            const replayList = resp || [];
            if (ok && replayList.length > 0) {
                nestedReplayList = [...nestedReplayList, replayList];
            } else {
                hasMoreReplays = false;
            }
        }
    }

    onMount(() => {
        const session = getClientSession();
        isDifferentUser = session !== undefined && user?.id !== session.userId;
        tryLoadReplays();
    });

    async function onSubmitCreateGame(timeControl: TimeControl, color: ColorSelect) {
        const { ok, status, resp, err } = await postCreateChallenge(timeControl, color);
        showCreateModal = false;
        console.error(ok, status, resp, err);
    }
</script>

<svelte:head>
    <title>{user ? user.username : "User"} - Hexchess</title>
</svelte:head>
<svelte:window onscroll={tryLoadReplays} />
<CreateGame title="Create a Challenge?" show={showCreateModal} onSubmit={onSubmitCreateGame} onClose={() => (showCreateModal = false)} />
<div class="center-horizontal-container">
    <div class="panel" style="width: 500px">
        {#if user}
            <div class="text-lg capped-size">{user.username}</div>
            <img class="flag-lg" src={`%sveltekit.assets%/flags/${user.country}.png`} alt="" />
            <br />

            <div class="panel-container" style="min-width: 450px; margin-bottom: 35px;">
                <div class="panel-elem">
                    <div class="panel-title">Rank</div>
                    <div class="panel-text">#{user.rank}</div>
                </div>
                <div class="panel-elem">
                    <div class="panel-title">Elo</div>
                    <div class="panel-text">{user.elo}</div>
                </div>
                <div class="panel-elem">
                    <div class="panel-title">Peak Elo</div>
                    <div class="panel-text">{user.highestElo}</div>
                </div>
            </div>

            <div class="panel-container" style="margin-bottom: 35px;">
                <div class="panel-elem">
                    <div class="panel-title">Win%</div>
                    <div class={`panel-text ${user.winRateColor}`}>{user.winRate}%</div>
                </div>
                <div class="panel-elem">
                    <div class="panel-title">Wins</div>
                    <div class="panel-text green-color">{user.wins}</div>
                </div>
                <div class="panel-elem">
                    <div class="panel-title">Losses</div>
                    <div class="panel-text red-color">{user.losses}</div>
                </div>
                <div class="panel-elem">
                    <div class="panel-title">Total</div>
                    <div class="panel-text">{user.total}</div>
                </div>
            </div>

            <div class="panel-container" style="margin-bottom: 0">
                <div class="panel-elem">
                    <div class="panel-title">Joined On</div>
                    <div class="panel-text">{user.joinedOn}</div>
                </div>
            </div>

            {#if user.bio}
                <div class="panel-container" style="margin-top: 35px;">
                    <div class="panel-elem">
                        <div class="panel-title" style="margin-bottom: 5px">Biography</div>
                        <div class="panel-text" style="font-size: 16px">{user.bio}</div>
                    </div>
                </div>
            {/if}
        {/if}

        <div id="profile-buttons" style="margin-top: 25px" style:display={isDifferentUser ? '' : 'none'}>
            <button class="button button-grey" id="challenge-button" onclick={() => (showCreateModal = true)}>
                <span class="svg-container">
                    <span style="margin-right: 8px">Challenge</span>
                    <ChallengeSvg />
                </span>
            </button>
        </div>
    </div>
</div>

<div class="center-horizontal-container" style="margin-top: 50px; margin-bottom: 50px;">
    <div class="wrapper">
        <table class="table-container" id="replay-table">
            <thead>
                <tr>
                    <th>White</th>
                    <th>Black</th>
                    <th>Result</th>
                    <th>Played On</th>
                </tr>
            </thead>
            <tbody id="replay-table-tbody">
                {#each nestedReplayList as replayList (replayList)}
                    {#each replayList as replay (replay.id)}
                        <tr id={'replay-' + replay.id} data-id={replay.id} class="row-hover" onclick={() => goto(`/games/replay/${replay.id}`)}>
                            <td style="width: 25%">
                                <a href={`/players/${replay.whiteId}`} class="text-ul">{replay.whiteName}</a>
                                <img class="flag" src={`%sveltekit.assets%/flags/${replay.whiteCountry}.png`} alt="" />
                                <span class={replay.whiteEloColor}>{replay.whiteEloDiff}</span>
                            </td>

                            <td style="width: 25%">
                                <a href={`/players/${replay.blackId}`} class="text-ul">{replay.blackName}</a>
                                <img class="flag" src={`%sveltekit.assets%/flags/${replay.blackCountry}.png`} alt="" />
                                <span class={replay.blackEloColor}>{replay.blackEloDiff}</span>
                            </td>

                            <td style="width: 25%">{replay.result}</td>
                            <td style="width: 25%">{replay.playedOn}</td>
                        </tr>
                    {/each}
                {/each}
            </tbody>
        </table>
    </div>
</div>

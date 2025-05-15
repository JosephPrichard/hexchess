<script lang="ts">
    import type { ColorSelect, TimeControl, UserWithReplaysView } from '$lib/models.js';
    import CreateGame from '$lib/components/modals/CreateGame.svelte';
    import { getReplays, postCreateChallenge, unwrap } from '$lib/api';
    import { onMount } from 'svelte';
    import ChallengeSvg from '$lib/components/icons/ChallengeIcon.svelte';
    import { formatReplayResult, getResultClasses, getWinrateClass } from '$lib/format.js';
    import { getClientSession } from '$lib/local';
    import Banner from '$lib/components/Banner.svelte';
    import { goto } from '$app/navigation';
    import { getNotificationsContext } from '$lib/context';
    import { createMessage } from '$lib/error';

    export interface PlayerProps {
        userWithReplays: UserWithReplaysView;
    }

    const { data: props }: { data: PlayerProps } = $props();
    const { user } = $derived(props.userWithReplays);

    const { addNotification } = getNotificationsContext();

    let nestedReplayList = $state([props.userWithReplays.replayList]);
    let showCreateModal = $state(false);
    let hasMoreReplays = $state(true);
    let isDifferentUser = $state(false);

    async function tryLoadReplays() {
        const lastId = nestedReplayList.at(-1)?.at(-1)?.id;
        const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight;
        const shouldLoadReplays = hasMoreReplays && isAtPageBottom && lastId !== undefined;

        if (shouldLoadReplays) {
            const { ok, resp, err } = await unwrap(getReplays(user.id, lastId));
            if (!ok) {
                console.error(ok, resp, err);
            }

            const replayList = resp || [];
            console.log(`Loaded ${replayList.length} new replays`);

            if (ok && replayList.length > 0) {
                nestedReplayList.push(replayList);
                console.log(`There are ${nestedReplayList.length} replayList records in the nestedReplayList`);
            } else {
                hasMoreReplays = false;
            }
        }
    }

    onMount(() => {
        const client = getClientSession();
        isDifferentUser = client !== null && client?.id !== client.id;
        tryLoadReplays();
    });

    async function onSubmitCreateChallenge(timeControl: TimeControl, color: ColorSelect) {
        const { ok, status, resp, err } = await unwrap(postCreateChallenge(timeControl, color));
        showCreateModal = false;
        console.error(ok, status, resp, err);

        if (ok || resp) {
            addNotification({ type: 'string', message: `Successfully created the challenge against ${user.username}`, isSuccess: false, duration: 3000 });
        } else {
            const message = createMessage(err);
            addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
        }
    }
</script>

<svelte:head>
    <title>{user ? user.username : "User"} - Hexchess</title>
</svelte:head>
<svelte:window onscroll={tryLoadReplays} />
<Banner />
<CreateGame title="Create a Challenge?" show={showCreateModal} onSubmit={onSubmitCreateChallenge} onClose={() => (showCreateModal = false)} />
<div class="center-horizontal-container">
    <div class="panel" style="width: 500px">
        <div class="text-lg capped-size">{user.username}</div>
        <img class="flag-lg" src={`/flags/${user.country}.png`} alt="" />
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
                <div class={`panel-text ${getWinrateClass(user.winRate)}`}>{user.winRate}%</div>
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
    {#if (nestedReplayList[0] || []).length > 0}
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
                <tbody>
                {#each nestedReplayList as replayList, i (i)}
                    {#each replayList as replay, i (i)}
                        {@const [whiteClass, blackClass] = getResultClasses(replay.result)}
                        <tr class="row-hover" onclick={() => goto(`/replay/${replay.id}`)}>
                            <td style="width: 25%">
                                <a href="/players/{replay.whiteId}" class="text-ul">{replay.whiteName}</a>
                                <img class="flag" src="/flags/{replay.whiteCountry}.png" alt="" />
                                <span class={whiteClass}>
                                    {replay.whiteEloDiff}
                                </span>
                            </td>
                            <td style="width: 25%">
                                <a href="/players/{replay.blackId}" class="text-ul">{replay.blackName}</a>
                                <img class="flag" src="/flags/{replay.blackCountry}.png" alt="" />
                                <span class={blackClass}>
                                    {replay.blackEloDiff}
                                </span>
                            </td>
                            <td style="width: 25%">
                                {formatReplayResult(replay.result)}
                            </td>
                            <td style="width: 25%">
                                {replay.playedOn}
                            </td>
                        </tr>
                    {/each}
                {/each}
                </tbody>
            </table>
        </div>
    {:else}
        <div class="color-wrapper" style="width: 530px;">
            This player hasn't played any games yet.
        </div>
    {/if}
</div>

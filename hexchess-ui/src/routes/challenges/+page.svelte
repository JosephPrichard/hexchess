<script lang="ts">
    import type { ChallengeView } from '$lib/models.js';
    import { postUpdateChallenge } from '$lib/api';

    export interface ChallengeProps {
        isSender: boolean;
        challengeList: ChallengeView[];
    }

    const { data }: { data: ChallengeProps } = $props();
    const { isSender, challengeList } = data;

    async function onUpdateChallenge(challenge: ChallengeView, action: string) {
        const { ok, status, resp, err } = await postUpdateChallenge(challenge.challengerId, challenge.challengeeId, action);
        console.error(ok, status, resp, err);
    }
</script>

<svelte:head>
    <title>Challenges - Hexchess</title>
</svelte:head>
<div class="bottom-right-anchor notifications-box" id="notifications-box" style="width: 350px"></div>
<div class="center-horizontal-container" style="margin-bottom: 100px">
    <div style="width: 600px;">
        <div class="tabs-group">
            <a class="tab" class:tab-selected={!isSender} href="?participants=received"> Received </a>
            <a class="tab" class:tab-selected={isSender} href="?participants=sent"> Sent </a>
        </div>

        <div class="challenge-list" id="challenge-list" style="display: {challengeList.length ? 'block' : 'none'}">
            {#each challengeList as challenge, index (index)}
                <div class="challenge-box">
                    <div>
                        <div style="margin-bottom: 6px">
                            <a href="/players/{challenge.challengerId}" class="text-ul bold-link">{challenge.challengerName}</a>
                            {#if !isSender}
                                <img class="flag" src={`%sveltekit.assets%/flags/{challenge.challengerCountry}.png"`} alt="" />
                                <b>({challenge.challengerElo})</b>
                            {/if}
                            vs
                            <a href="/players/{challenge.challengeeId}" class="text-ul bold-link">{challenge.challengeeName}</a>
                            {#if isSender}
                                <img class="flag" src={`%sveltekit.assets%/flags/${challenge.challengeeCountry}.png`} alt="" />
                                <b>({challenge.challengeeElo})</b>
                            {/if}
                        </div>
<!--						<div style="margin-bottom: 6px">-->
<!--							<b>{challenge.mode}</b>-->
<!--						</div>-->
                        <div style="margin-bottom: 6px">
                            Sent {challenge.madeAgo}
                        </div>
                        <div style="margin-bottom: 6px">
                            Expires {challenge.expiresIn}
                        </div>
                    </div>
                    <div class="center-relative">
                        <div class="vertical-align">
                            {#if isSender}
                                <button id="challenge-{index}-delete" class="button-small button-small-red" style="margin-left: 5px" onclick={() => onUpdateChallenge(challenge, 'DELETE')}>
                                    Delete
                                </button>
                            {:else}
                                <button id="challenge-{index}-accept" class="button-small button-small-green" style="margin-left: 5px" onclick={() => onUpdateChallenge(challenge, 'ACCEPT')}>
                                    Accept
                                </button>
                                <button id="challenge-{index}-reject" class="button-small button-small-red" style="margin-left: 5px" onclick={() => onUpdateChallenge(challenge, 'REJECT')}>
                                    Reject
                                </button>
                            {/if}
                        </div>
                    </div>
                </div>
            {/each}
        </div>

        <div class="no-challenges" id="no-challenges" style="display: {challengeList.length ? 'none' : 'block'}">
            {#if isSender}
                No challenges have been sent
            {:else}
                No challenges have been received
            {/if}
        </div>
    </div>
</div>

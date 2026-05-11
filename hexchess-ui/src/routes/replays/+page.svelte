<script lang="ts">
    import Banner from "$lib/Banner.svelte";
    import {
        type GameMode, type ReplayCause, type ReplayModel, type ReplayQuerySortKey, type ReplayResult,
        GameModeOptions, ReplayCauseOptions, ReplayQuerySortKeyOptions, ReplayResultOptions, ReplayQuerySortKeyNameMap,
    } from "$lib/api/models";
    import ReplayPreview from "$lib/components/ReplayPreview.svelte";
    import services from "$lib/api/services";
    import Dropdown from "$lib/components/Dropdown.svelte";
    import {goto} from "$app/navigation";
    import {getNotificationsContext} from "$lib/utils/context";
    import UserAutocompleteInput from "$lib/components/UserAutocompleteInput.svelte";
    import {defaultOption, expectEnum, mapReplaysPropsToQuery, mapReplaysPropsToURLParams, type OptEnum, parseEnum, type ReplaysSearchProps} from "./service";

    const modeOptions = [defaultOption<OptEnum<GameMode>>(), ...GameModeOptions];
    const causeOptions = [defaultOption<OptEnum<ReplayCause>>(), ...ReplayCauseOptions];
    const resultOptions = [defaultOption<OptEnum<ReplayResult>>(), ...ReplayResultOptions];

    export interface ReplaysProps {
        replays: ReplayModel[];
        search?: ReplaysSearchProps;
    }

    const { addErrorNotification } = getNotificationsContext();

    const { data: props }: { data: ReplaysProps } = $props();

    let replayList: ReplayModel[] = $state(props.replays);
    let hasMoreReplays = $state(true);

    let search = props.search ?? null;

    let winnername: string = $state(search?.winnername ?? "");
    let losername: string = $state(search?.losername ?? "");
    let whitename: string = $state(search?.whitename ?? "");
    let blackname: string = $state(search?.blackname ?? "");

    let fromDate: string = $state(search?.fromDate ?? "");
    let toDate: string = $state(search?.toDate ?? "");
    let mode: OptEnum<GameMode> = $state(search?.mode ?? "");
    let cause: OptEnum<ReplayCause> = $state(search?.cause ?? "");
    let result: OptEnum<ReplayResult> = $state(search?.result ?? "");

    let sort: ReplayQuerySortKey = $state(expectEnum(search?.sort, ReplayQuerySortKeyNameMap, "id"))

    $effect(() => {
        // keeps the local "draft" up to date whenever we receive a new version of the replays search result from the server
        replayList = props.replays;
    });

    async function onRerouteSearch() {
        const params = mapReplaysPropsToURLParams({
            winnername, losername, whitename, blackname, fromDate, toDate, mode, cause, result, sort
        });
        hasMoreReplays = true
        await goto(`/replays?${params}`);
    }

    async function tryLoadReplays() {
        const lastReplay = replayList.at(-1);

        const lastId = lastReplay?.id;
        const lastRating = lastReplay?.rating;
        const lastTurnCount = lastReplay?.turnCount;

        const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight - 100;
        const shouldLoadReplays = hasMoreReplays && isAtPageBottom && lastId !== undefined;

        if (shouldLoadReplays) {
            const replayQuery = mapReplaysPropsToQuery(props.search, {afterId: lastId, afterTurnCount: lastTurnCount, afterRating: lastRating});
            const [data, err] = await services.getReplays(replayQuery, fetch);
            if (data) {
                const nextReplayList = data?.replayList ?? [];

                if (nextReplayList.length > 0) {
                    replayList.push(...nextReplayList);
                } else {
                    hasMoreReplays = false;
                }
            } else {
                addErrorNotification(err);
            }
        }
    }
</script>

<svelte:head>
    <title>Replays - Hexchess</title>
</svelte:head>
<Banner />
<svelte:window onscroll={tryLoadReplays} />
<div class="center-horizontal-container">
    <div class="title-md">
        Search Replays
    </div>
    <div class="wrapper">
        <div class="search-wrapper">
            <form class="search-form" onsubmit={e => e.preventDefault()}>
                <div class="sw-child">
                    <div class="sw-input">
                        <label for="from-date" class="sw-title">From</label>
                        <div class="search-input">
                            <input name="from-date" type="date" bind:value={fromDate} />
                        </div>
                    </div>

                    <div class="sw-input">
                        <label for="to-date" class="sw-title">To</label>
                        <div class="search-input">
                            <input name="to-date" type="date" bind:value={toDate} />
                        </div>
                    </div>

                    <div class="sw-input">
                        <label for="winnerName" class="sw-title">Winner</label>
                        <div class="search-input-large">
                            <UserAutocompleteInput inputName="winnerName" bind:username={winnername}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <label for="loserName" class="sw-title">Loser</label>
                        <div class="search-input-large">
                            <UserAutocompleteInput inputName="loserName" bind:username={losername}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <label for="whiteName" class="sw-title">White</label>
                        <div class="search-input-large">
                            <UserAutocompleteInput inputName="whiteName" bind:username={whitename}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <label for="blackName" class="sw-title">Black</label>
                        <div class="search-input-large">
                            <UserAutocompleteInput inputName="blackName" bind:username={blackname}/>
                        </div>
                    </div>
                </div>
                <div class="sw-child">
                    <div class="sw-input">
                        <div class="sw-title">Mode</div>
                        <div class="search-dropdown">
                            <Dropdown options={modeOptions} selected={mode} onChange={value => mode = value}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <div class="sw-title">Cause</div>
                        <div class="search-dropdown">
                            <Dropdown options={causeOptions} selected={cause} onChange={value => cause = value}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <div class="sw-title">Result</div>
                        <div class="search-dropdown">
                            <Dropdown options={resultOptions} selected={result} onChange={value => result = value}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <div class="sw-title">Sort</div>
                        <div class="search-dropdown">
                            <Dropdown options={ReplayQuerySortKeyOptions} selected={sort} onChange={value => sort = value}/>
                        </div>
                    </div>
                </div>
            </form>
            <div class="search-button-wrapper">
                <button type="submit" class="button-small button-small-green" onclick={onRerouteSearch}>
                    Search
                </button>
            </div>
        </div>
        {#if (replayList?.length ?? 0) === 0}
            <div class="color-wrapper no-replays-wrapper">
                No replays match the search criterea.
            </div>
        {:else}
            <div class="replay-snippets-wrapper">
                {#each replayList as replay, index (index)}
                    {@const rounding = function() {
                        if (index === 0) {
                            return "rounded-top";
                        } else if (index === replayList.length - 1) {
                            return "rounded-bottom";
                        } else {
                            return undefined;
                        }
                    }()}
                    <ReplayPreview index={index} replay={replay} rounding={rounding} />
                {/each}
            </div>
        {/if}
    </div>
</div>

<style>
    .no-replays-wrapper {
        margin-top: 30px;
    }

    .search-button-wrapper {
        text-align: right;
    }

    .search-wrapper {
        display: flex;
        flex-direction: column;
        gap: 20px;
    }

    .search-form {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;
        gap: 40px;
        overflow: visible;
        height: fit-content;
    }

    @media (max-width: 768px) {
        .search-form {
            max-width: 400px;
            flex-direction: column;
        }
    }

    .sw-title {
        font-size: 14px;
        width: fit-content;
        flex: 0.2;
    }

    .sw-input {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;
        gap: 15px;
        width: 100%;
    }

    .sw-child {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: 10px;
    }

    .search-input {
        font-size: 14px;
        flex: 0.8;
    }

    .search-dropdown {
        flex: 0.8;
        min-width: 200px;
    }

    .search-input-large {
        flex: 0.8;
    }

    @media (max-width: 768px) {
        .search-input {
            min-width: 0;
            width: 100%;
        }
        .search-input-large {
            min-width: 0;
            width: 100%;
        }
    }

    .replay-snippets-wrapper {
        margin-top: 20px;
        margin-bottom: 20px;
    }
</style>
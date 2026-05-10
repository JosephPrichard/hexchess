<script lang="ts">
    import Banner from "$lib/Banner.svelte";
    import {
        type GameMode,
        GameModeOptions,
        type ReplayCause,
        ReplayCauseOptions,
        type ReplayModel, type ReplayQuerySortKey, ReplayQuerySortKeyOptions,
        type ReplayResult,
        ReplayResultOptions, GameModeNameMap, ReplayCauseNameMap, ReplayQuerySortKeyNameMap, ReplayResultNameMap
    } from "$lib/api/models";
    import ReplayPreview from "$lib/components/ReplayPreview.svelte";
    import services, {type ReplaysQuery} from "$lib/api/services";
    import Dropdown from "$lib/components/Dropdown.svelte";
    import {goto} from "$app/navigation";
    import {getNotificationsContext} from "$lib/utils/context";
    import UserAutocompleteInput from "$lib/components/UserAutocompleteInput.svelte";

    type Optional<T> = T | "";

    const parseEnum = <T extends string>(value: string | undefined, nameMap: Record<T, string>): Optional<T> =>
        value !== "" && nameMap[value as T] !== undefined ? value as T : ""

    const expectEnum = <T extends string>(value: string | undefined, nameMap: Record<T, string>, fallback: T): T =>
        value !== "" && nameMap[value as T] !== undefined ? value as T : fallback

    const defaultOption = <T extends string>() =>
        ({ value: "" as T, label: "" });

    const modeOptions = [defaultOption<Optional<GameMode>>(), ...GameModeOptions];
    const causeOptions = [defaultOption<Optional<ReplayCause>>(), ...ReplayCauseOptions];
    const resultOptions = [defaultOption<Optional<ReplayResult>>(), ...ReplayResultOptions];

    export interface ReplaysProps {
        replays: ReplayModel[];
        query?: ReplaysQuery;
    }

    const { addErrorNotification } = getNotificationsContext();

    const { data: props }: { data: ReplaysProps } = $props();

    const q = props.query;

    let replayList: ReplayModel[] = $state(props.replays);
    let hasMoreReplays = $state(true);

    let fromDate: string = $state(q?.fromDate ?? "");
    let toDate: string = $state(q?.toDate ?? "");
    let mode: Optional<GameMode> = $state(parseEnum(q?.mode, GameModeNameMap));
    let cause: Optional<ReplayCause> = $state(parseEnum(q?.cause, ReplayCauseNameMap));
    let result: Optional<ReplayResult> = $state(parseEnum(q?.result, ReplayResultNameMap));
    let winnername: string = $state(q?.winnername ?? "");
    let losername: string = $state(q?.losername ?? "");
    let whitename: string = $state(q?.whitename ?? "");
    let blackname: string = $state(q?.blackname ?? "");

    let sort: ReplayQuerySortKey = $state(expectEnum(q?.sort, ReplayQuerySortKeyNameMap, "id"))

    $effect(() => {
        // keeps the local "draft" up to date whenever we receive a new version of the replays search result from the server
        replayList = props.replays;
    });

    async function onSearch() {
        const params = new URLSearchParams();
        const paramsObj: ReplaysQuery = {
            fromDate, toDate, mode, cause, result, winnername, losername, whitename, blackname, sort
        };
        for (let [key, value] of Object.entries(paramsObj)) {
            if (value !== "") {
                params.set(key, value);
            }
        }
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
            const replayQuery: ReplaysQuery = {...props.query, afterId: lastId, afterRating: lastRating, afterTurnCount: lastTurnCount};

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
                <button type="submit" class="button-small button-small-green" onclick={onSearch}>
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
<script lang="ts">
    import Banner from "$lib/Banner.svelte";
    import {
        type GameMode,
        GameModeOptions,
        type ReplayCause,
        ReplayCauseOptions,
        type ReplayModel,
        type ReplayResult,
        ReplayResultOptions, TypedGameModeNameMap, TypedReplayCauseNameMap, TypedReplayResultNameMap
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

    let nestedReplayList: ReplayModel[][] = $state([props.replays]);
    let hasMoreReplays = $state(true);

    let fromDate: string = $state(q?.fromDate ?? "");
    let toDate: string = $state(q?.toDate ?? "");
    let mode: Optional<GameMode> = $state(parseEnum(q?.mode, TypedGameModeNameMap));
    let cause: Optional<ReplayCause> = $state(parseEnum(q?.cause, TypedReplayCauseNameMap));
    let result: Optional<ReplayResult> = $state(parseEnum(q?.result, TypedReplayResultNameMap));
    let winnerName: string = $state(q?.winnername ?? "");
    let loserName: string = $state(q?.losername ?? "");
    let whiteName: string = $state(q?.whitename ?? "");
    let blackName: string = $state(q?.blackname ?? "");

    $effect(() => {
        // keeps the local "draft" up to date whenever we receive a new version of the replays search result from the server
        nestedReplayList = [props.replays];
    });

    async function onSearch() {
        const params = new URLSearchParams();
        const paramsObj: ReplaysQuery = { fromDate, toDate, mode, cause, result,
            winnername: winnerName, losername: loserName, whitename: whiteName, blackname: blackName };
        for (let [key, value] of Object.entries(paramsObj)) {
            if (value !== "") {
                params.set(key, value);
            }
        }
        await goto(`/replays?${params}`);
    }

    async function tryLoadReplays() {
        const lastId = nestedReplayList.at(-1)?.at(-1)?.id;
        const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight - 100;
        const shouldLoadReplays = hasMoreReplays && isAtPageBottom && lastId !== undefined;

        if (shouldLoadReplays) {
            const replayQuery = {...props.query, afterId: lastId}

            const [data, err] = await services.getReplays(replayQuery, fetch);
            if (data) {
                const nextReplayList = data?.replayList ?? [];

                if (nextReplayList.length > 0) {
                    nestedReplayList.push(nextReplayList);
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
                        <input name="from-date" type="date" class="search-input" bind:value={fromDate} />
                    </div>

                    <div class="sw-input">
                        <label for="to-date" class="sw-title">To</label>
                        <input name="to-date" type="date" class="search-input" bind:value={toDate} />
                    </div>

                    <div class="sw-input">
                        <label for="mode" class="sw-title">Mode</label>
                        <Dropdown options={modeOptions} selected={mode} onChange={value => mode = value}/>
                    </div>

                    <div class="sw-input">
                        <label for="cause" class="sw-title">Cause</label>
                        <Dropdown options={causeOptions} selected={cause} onChange={value => cause = value}/>
                    </div>

                    <div class="sw-input">
                        <label for="result" class="sw-title">Result</label>
                        <Dropdown options={resultOptions} selected={result} onChange={value => result = value}/>
                    </div>
                </div>
                <div class="sw-child">
                    <div class="sw-input">
                        <label for="winnerName" class="sw-title">Winner</label>
                        <div class="search-input-large">
                            <UserAutocompleteInput inputName="winnerName" bind:username={winnerName}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <label for="loserName" class="sw-title">Loser</label>
                        <div class="search-input-large">
                            <UserAutocompleteInput inputName="loserName" bind:username={loserName}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <label for="whiteName" class="sw-title">White</label>
                        <div class="search-input-large">
                            <UserAutocompleteInput inputName="whiteName" bind:username={whiteName}/>
                        </div>
                    </div>

                    <div class="sw-input">
                        <label for="blackName" class="sw-title">Black</label>
                        <div class="search-input-large">
                            <UserAutocompleteInput inputName="blackName" bind:username={blackName}/>
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
        {#if (nestedReplayList?.length ?? 0) === 0 || nestedReplayList[0].length === 0}
            <div class="color-wrapper no-replays-wrapper">
                No replays match the search criterea.
            </div>
        {:else}
            <div class="replay-snippets-wrapper">
                {#each nestedReplayList as replayList}
                    {#each replayList as replay, index (index)}
                        {@const rounding = function() {
                            if (index === 0) {
                                return "rounded-top";
                            } else if (index === replayList.length - 1) {
                                return "rounded-bottom";
                            } else {
                                return "";
                            }
                        }()}
                        <ReplayPreview index={index} replay={replay} rounding={rounding} />
                    {/each}
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
        height: 25px;
        font-size: 14px;
        flex: 0.8;
        width: 150px;
        padding-left: 15px;
        padding-right: 15px;
    }

    .search-input-large {
        flex: 0.8;
        min-width: 250px;
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
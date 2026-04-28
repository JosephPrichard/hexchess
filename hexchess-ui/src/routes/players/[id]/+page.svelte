<script lang="ts">
	import CreateGame from '$lib/components/modals/CreateGame.svelte';
	import { onMount } from 'svelte';
	import ChallengeIcon from '$lib/components/icons/ChallengeIcon.svelte';
	import {type EloBuckets, type ReplayModel} from '$lib/api/models.js';
	import { getClientSession } from '$lib/utils/storage';
	import { getNotificationsContext } from '$lib/utils/context';
	import type { ColorSelect, FullPlayerModel, GameMode } from '$lib/api/models';
	import services from '$lib/api/services';
	import 'chartjs-adapter-date-fns';
	import '$lib/utils/chart';
	import Dropdown from '$lib/components/util/Dropdown.svelte';
	import { formatJoinedOn, getWinrateClass } from '$lib/utils/format';
	import Banner from '$lib/Banner.svelte';
	import ProfilePic from '$lib/components/user/ProfilePic.svelte';
	import {makeEloHistoriesChart} from "./chart";
	import ReplaySnippet from "$lib/components/user/ReplaySnippet.svelte";
	import UserStats from "$lib/components/user/UserStats.svelte";
	import {getReplay, nonDefaultQueries, type ReplayQueryKind, replayQueryOptions} from "./service";
	import { goto } from '$app/navigation';
	import SettingsIcon from "$lib/components/icons/SettingsIcon.svelte";

	const timeframes: { label: string, value: string }[] = [
		{ label: "All Time", value: "all" },
		{ label: "1 Year", value: "1y" },
		{ label: "6 Months", value: "6m" },
		{ label: "3 Months", value: "3m" },
		{ label: "1 Month", value: "1m" },
	];

	export interface PlayerProps {
		fullUser: FullPlayerModel;
	}

	const { data: props }: { data: PlayerProps } = $props();
	const { user, stats: userStats } = $derived(props.fullUser);

	const { addNotification, addErrorNotification } = getNotificationsContext();

	interface ReplayListRow {
		nestedReplayList: ReplayModel[][];
		hasMoreReplays: boolean;
	}

	let replayLists: Record<ReplayQueryKind, ReplayListRow> = $state({
		"allReplays": { nestedReplayList: [props.fullUser.replayList], hasMoreReplays: true },
		"wonReplays": { nestedReplayList: [], hasMoreReplays: true },
		"lostReplays": { nestedReplayList: [], hasMoreReplays: true },
	});
	let replayQueryKind: ReplayQueryKind = $state("allReplays");
	let showCreateModal = $state(false);
	let isDifferentUser = $state(false);
	let timeframeIndex = $state(0);

	let totalGames = $derived(userStats.totalWins + userStats.totalLosses + userStats.totalDraws);

	let chartElement: HTMLCanvasElement;

	async function tryLoadReplays() {
		const replayListRow = replayLists[replayQueryKind];
		if (!nestedReplayList) {
			console.error(`No replay list found for active replays query: ${replayQueryKind}.`);
			return;
		}

		const lastId = replayListRow.nestedReplayList.at(-1)?.at(-1)?.id;
		const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight - 100;
		const shouldLoadReplays = replayListRow.hasMoreReplays && isAtPageBottom && lastId !== undefined;

		if (shouldLoadReplays) {
			const [data, err] = await getReplay(replayQueryKind, lastId, user?.id);
			if (err) {
				addErrorNotification(err);
				return;
			}
			const replayList = data?.replayList ?? [];

			if (replayList.length > 0) {
				replayListRow.nestedReplayList.push(replayList);
			} else {
				replayListRow.hasMoreReplays = false;
			}
		}
	}

	async function loadEloHistories(userId: number, timeframe: string, onLoaded: (buckets: Record<string, EloBuckets>) => void) {
		const [data, err] = await services.getEloHistories(userId, timeframe);
		if (data) {
			onLoaded(data.buckets);
		} else {
			addErrorNotification(err);
		}
	}

	$effect(() => {
		const ctx = chartElement.getContext("2d");
		if (!ctx) return;
		let chart: any;

		const timeframe = timeframes[timeframeIndex].value;

		loadEloHistories(user.id, timeframe, (buckets) => {
			chart = makeEloHistoriesChart(ctx, buckets);
		});
		return () => {
			if (chart) chart.destroy();
		};
	});

	function loadForAllReplayQueries(userId: number) {
		for (const replayQueryKind of nonDefaultQueries) {
			getReplay(replayQueryKind, undefined, userId).then(([data, err]) => {
				if (err) {
					addErrorNotification(err);
					return;
				}
				replayLists[replayQueryKind].nestedReplayList = data?.replayList ? [data?.replayList] : [];
			});
		}
	}

	$effect(() => {
		loadForAllReplayQueries(user.id);
	})

	onMount(() => {
		const client = getClientSession();
		isDifferentUser = client !== null && user.id !== client.id;
		console.log('Initializing with client: ', client);
		tryLoadReplays();
	});

	async function onSubmitCreateChallenge(timeControl: GameMode, color: ColorSelect) {
		const [data, err] = await services.postCreateChallenge(timeControl, color, user.id);
		if (data) {
			addNotification({
				type: 'string',
				message: `Successfully created the challenge against ${user.username}`,
				isSuccess: true,
				duration: 3000
			});
		} else {
			addErrorNotification(err);
		}
		showCreateModal = false;
	}

	const nestedReplayList = $derived(replayLists[replayQueryKind].nestedReplayList || []);
</script>

<svelte:head>
	<title>{user ? user.username : 'User'} - Hexchess</title>
</svelte:head>
<svelte:window onscroll={tryLoadReplays} />
<Banner />
<CreateGame title="Create a Challenge?" bind:show={showCreateModal} onSubmit={onSubmitCreateChallenge} />
<div class="center-horizontal-container">
	<div class="panel" style="padding: 0">
		<div style="padding: 20px">
			<ProfilePic userId={user.id} size={125}/>
			<div class="text-lg capped-size">{user.username}</div>
			<img class="flag-lg" src={`/flags/${user.country}.png`} alt="" />
			<br />

			<div class="panel-container">
				<div class="panel-elem">
					<div class="panel-title">Joined On</div>
					<div class="panel-text">{formatJoinedOn(user.joinedOn)}</div>
				</div>
			</div>

			{#if user.bio}
				<div class="panel-container bio">
					<div class="panel-elem">
						<div class="panel-title" style="margin-bottom: 5px">Biography</div>
						<div class="panel-text" style="font-size: 16px">{user.bio}</div>
					</div>
				</div>
			{/if}

			{#if isDifferentUser}
				<div style="margin-bottom: 25px;">
					<button class="button button-grey" id="challenge-button" onclick={() => (showCreateModal = true)}>
					<span class="svg-container">
						<span style="margin-right: 8px">Challenge</span>
						<ChallengeIcon />
					</span>
					</button>
				</div>
			{/if}

			<h3>
				Total Stats
			</h3>
			<div class="panel-container">
				<div class="panel-elem">
					<div class="panel-title">Elo</div>
					<div class="panel-text">{Math.round(userStats.avgElo)}</div>
				</div>
				<div class="panel-elem">
					<div class="panel-title">Peak Elo</div>
					<div class="panel-text">{Math.round(userStats.highestElo)}</div>
				</div>
				<div class="panel-elem">
					<div class="panel-title">Win%</div>
					<div class="panel-text {getWinrateClass(userStats.totalWinrate)}">{userStats.totalWinrate}%</div>
				</div>
				<div class="panel-elem">
					<div class="panel-title">Won</div>
					<div class="panel-text green-color">{userStats.totalWins}</div>
				</div>
				<div class="panel-elem">
					<div class="panel-title">Lost</div>
					<div class="panel-text red-color">{userStats.totalLosses}</div>
				</div>
				<div class="panel-elem">
					<div class="panel-title">Drawn</div>
					<div class="panel-text yellow-color">{userStats.totalDraws}</div>
				</div>
				<div class="panel-elem">
					<div class="panel-title">Total</div>
					<div class="panel-text">{totalGames}</div>
				</div>
			</div>

			<div style="margin-bottom: 35px;">
				<h3>
					Stats by Mode
				</h3>
				<UserStats userStats={userStats}/>
			</div>

			<h3>
				Stats Over Time
			</h3>
			<div class="dropdown-wrapper">
				<Dropdown
						options={timeframes}
						selected={timeframes[timeframeIndex].value}
						onChange={value => timeframeIndex = timeframes.findIndex((t) => t.value === value)}
				/>
			</div>
			<div class="canvas-wrapper">
				<canvas bind:this={chartElement} width="650px" height="250px"></canvas>
			</div>
		</div>

		<div class="game-histories-tab-group">
			{#each replayQueryOptions as tab, i (i)}
				<button class="game-histories-tab"
						class:active-game-histories-tab={replayQueryKind === tab.value}
						onclick={() => replayQueryKind = tab.value}
				>
					{tab.label}
				</button>
			{/each}
		</div>
		<div class="game-histories">
			{#if (nestedReplayList[0] ?? []).length > 0}
				{#each nestedReplayList as replayList, i (i)}
					{#each replayList as replay, i (i)}
						<ReplaySnippet replay={replay} index={i}/>
					{/each}
				{/each}
			{:else}
				<div class="no-replays-wrapper">
					<div class="color-wrapper" style="width: 530px;">No games match the search category.</div>
				</div>
			{/if}
		</div>
	</div>
</div>

<style>
	.button-profile {
		display: flex;
		align-items: center;
		padding-left: 20px;
		padding-right: 25px;
		min-width: fit-content;
		gap: 5px;
	}

	.preferences-wrapper {
		font-size: 18px;
		margin-top: 10px;
		margin-bottom: 50px;
	}

	.no-replays-wrapper {
		margin-top: 25px;
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.game-histories-tab-group {
		display: flex;
		flex-direction: row;
	}

	.game-histories-tab:first-child {
		border-left: 1px solid rgb(58, 58, 58);
	}

	.game-histories-tab {
		cursor: pointer;

		margin: 0;
		padding: 15px 35px;
		width: fit-content;

		border-top-left-radius: 5px;
		border-top-right-radius: 5px;
		background-color: rgb(50, 50, 50);
		border: 1px solid rgb(58, 58, 58);
		border-left: none;

		transition: background-color 0.2s ease-in-out;
	}

	.game-histories-tab:hover {
		background-color: rgb(60, 60, 60);
	}

	.active-game-histories-tab {
		background-color: rgb(38, 38, 38);
	}

	.active-game-histories-tab:hover {
		background-color: rgb(38, 38, 38);
	}

	.game-histories {
		margin-bottom: 20px;
	}

	.dropdown-wrapper {
		margin-top: 25px;
		margin-bottom: 25px;
		width: 33%;
		min-width: 200px;
		font-size: 12px;
	}

	.canvas-wrapper {
		margin-top: 25px;
		display: block;
	}

    .panel-container {
		min-width: 450px;
		margin-bottom: 35px;
        display: flex;
        flex-wrap: wrap;
        gap: 45px;
    }

    .panel-elem {
        font-size: 16px;
        box-sizing: border-box;
    }

    .panel-title {
        font-weight: 600;
        color: rgb(160, 160, 160);
    }

    .panel-text {
        font-size: 20px;
    }

	.bio {
		max-width: 750px;
	}
</style>
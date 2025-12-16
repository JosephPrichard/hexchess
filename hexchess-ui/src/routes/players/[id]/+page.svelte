<script lang="ts">
	import CreateGame from '$lib/components/modals/CreateGame.svelte';
	import { onMount } from 'svelte';
	import ChallengeIcon from '$lib/components/icons/ChallengeIcon.svelte';
	import { type EloBuckets,  ReplayModeMap, type Timeframe } from '$lib/api/models.js';
	import { getClientSession } from '$lib/utils/storage';
	import Banner from '$lib/Banner.svelte';
	import { goto } from '$app/navigation';
	import { getNotificationsContext } from '$lib/utils/context';
	import { makeMessage } from '$lib/utils/error';
	import type { ColorSelect, TimeControl, FullUserModel } from '$lib/api/models';
	import services from '$lib/api/services';
	import "chartjs-adapter-date-fns";
	import "$lib/utils/chart"
	import { Chart } from 'chart.js';
	import { typedEntries } from '$lib/utils/array';
	import { generateColors } from '$lib/utils/colors';
	import Dropdown from '$lib/components/util/Dropdown.svelte';
	import { formatEloDiff, formatJoinedOn, formatReplayResult, formatPlayedOn, normalizeToDay, formatTimestamp } from '$lib/utils/format';

	const timeframes: { label: string, value: Timeframe }[] = [
		{ label: "All Time", value: "all" },
		{ label: "1 Year", value: "1y" },
		{ label: "6 Months", value: "6m" },
		{ label: "3 Months", value: "3m" },
		{ label: "1 Month", value: "1m" },
	];

	export interface PlayerProps {
		fullUser: FullUserModel;
	}

	const { data: props }: { data: PlayerProps } = $props();
	const { user } = $derived(props.fullUser);

	const { addNotification } = getNotificationsContext();

	let nestedReplayList = $state([props.fullUser.replayList]);
	let showCreateModal = $state(false);
	let hasMoreReplays = $state(true);
	let isDifferentUser = $state(false);
	let timeframeIndex = $state(0);

	let chartElement: HTMLCanvasElement;

	async function tryLoadReplays() {
		const lastId = nestedReplayList.at(-1)?.at(-1)?.id;
		const isAtPageBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight;
		const shouldLoadReplays = hasMoreReplays && isAtPageBottom && lastId !== undefined;

		if (shouldLoadReplays) {
			const [data, err] = await services.getReplays(user.id, lastId);
			if (err) {
				console.error("Error loading replays: ", err);
				return;
			}
			const replayList = data?.replayList || [];
			console.log(`Loaded ${replayList.length} new replays`);

			if (replayList.length > 0) {
				nestedReplayList.push(replayList);
				console.log(`There are ${nestedReplayList.length} replayList records in the nestedReplayList`);
			} else {
				hasMoreReplays = false;
			}
		}
	}

	async function loadEloHistories(userId: number, timeframe: Timeframe, onLoaded: (buckets: EloBuckets) => void) {
		const [data, err] = await services.getEloHistories(userId, timeframe);
		if (data) {
			onLoaded(data.buckets);
		} else {
			const message = makeMessage(err);
			addNotification({ type: 'string', message, isSuccess: false });
		}
	}

	function makeEloHistoriesChart(ctx: CanvasRenderingContext2D, buckets: EloBuckets) {
		const entries = Object.entries({
			"ALL": buckets,
		});
		const colors = generateColors(entries.length);

		return new Chart(ctx, {
			type: "line",
			data: {
				datasets: entries.map(([mode, eloHistories], i) => ({
					label: ReplayModeMap[mode] || "Unknown Mode",
					data: eloHistories.map((h) => ({ x: normalizeToDay(h.timestamp), y: h.elo})),
					borderWidth: 2,
					tension: 0.25,
					pointRadius: 3,
					borderColor: colors[i](1),
					backgroundColor: colors[i](0.2),
					fill: true,
					pointBackgroundColor: colors[i](1)
				}))
			},
			options: {
				responsive: true,
				scales: {
					x: {
						type: "time",
						time: {
							minUnit: 'day',
						},
						ticks: {
							callback: formatTimestamp
						},
						title: {
							display: true,
							text: 'Date',
							font: {
								size: 14,
								weight: 'bold'
							},
							color: 'rgb(120,120,120)'
						}
					},
					y: {
						beginAtZero: false,
						ticks: {
							callback: (value) => String(value)
						},
						title: {
							display: true,
							text: 'Elo',
							font: {
								size: 14,
								weight: 'bold'
							},
							color: 'rgb(120,120,120)'
						}
					}
				}
			},
		});
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

	onMount(() => {
		const client = getClientSession();
		isDifferentUser = client !== null && user.id !== client.id;
		console.log('Initializing with client: ', client);
		tryLoadReplays();
	});

	async function onSubmitCreateChallenge(timeControl: TimeControl, color: ColorSelect) {
		const [data, err] = await services.postCreateChallenge(timeControl, color, user.id);
		showCreateModal = false;
		if (data) {
			addNotification({
				type: 'string',
				message: `Successfully created the challenge against ${user.username}`,
				isSuccess: true,
				duration: 3000
			});
		} else {
			const message = makeMessage(err);
			addNotification({ type: 'string', message, isSuccess: false });
		}
		showCreateModal = true;
	}

	const winrateClass = $derived.by(() => {
		if (user.winRate > 50) {
			return 'green-color';
		} else if (user.winRate < 50) {
			return 'red-color';
		} else {
			return 'yellow-color';
		}
	});
</script>

<svelte:head>
	<title>{user ? user.username : 'User'} - Hexchess</title>
</svelte:head>
<svelte:window onscroll={tryLoadReplays} />

<CreateGame title="Create a Challenge?" bind:show={showCreateModal} onSubmit={onSubmitCreateChallenge} />
<div class="center-horizontal-container">
	<div class="panel player-panel">
		<div class="text-lg capped-size">{user.username}</div>
		<img class="flag-lg" src={`/flags/${user.country}.png`} alt="" />
		<br />

		<div class="panel-container">
			<div class="panel-elem">
				<div class="panel-title">Rank</div>
				<div class="panel-text">#{user.rank}</div>
			</div>
			<div class="panel-elem">
				<div class="panel-title">Elo</div>
				<div class="panel-text">{Math.round(user.elo)}</div>
			</div>
			<div class="panel-elem">
				<div class="panel-title">Peak Elo</div>
				<div class="panel-text">{Math.round(user.highestElo)}</div>
			</div>
		</div>

		<div class="panel-container" style="margin-bottom: 35px;">
			<div class="panel-elem">
				<div class="panel-title">Win%</div>
				<div class="panel-text {winrateClass}">{user.winRate}%</div>
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
				<div class="panel-text">{formatJoinedOn(user.joinedOn)}</div>
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

		{#if isDifferentUser}
			<div style="margin-top: 25px">
				<button class="button button-grey" id="challenge-button" onclick={() => (showCreateModal = true)}>
					<span class="svg-container">
						<span style="margin-right: 8px">Challenge</span>
						<ChallengeIcon />
					</span>
				</button>
			</div>
		{/if}
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
</div>
<div class="center-horizontal-container" style="margin-top: 50px; margin-bottom: 50px;">
	{#if (nestedReplayList[0] || []).length > 0}
		<div class="wrapper">
			<table class="table-container">
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
							{@const [whiteClass, blackClass] = function() {
								switch (replay.result) {
								case 'WHITE_WINS':
									return ['green-color', 'red-color'];
								case 'BLACK_WINS':
									return ['red-color', 'green-color'];
								case 'DRAW':
									return ['yellow-color', 'yellow-color'];
								default:
									console.error('Unknown result case', replay.result);
									return ['', ''];
								}
							}()}
							<tr class="row-hover" onclick={() => goto(`/replay/${replay.id}`)}>
								<td style="width: 25%">
									<a href="/players/{replay.whiteId}" class="text-ul">{replay.whiteName}</a>
									<img class="flag" src="/flags/{replay.whiteCountry}.png" alt="" />
									<span class={whiteClass}>
										{formatEloDiff(replay.whiteEloDiff)}
									</span>
								</td>
								<td style="width: 25%">
									<a href="/players/{replay.blackId}" class="text-ul">{replay.blackName}</a>
									<img class="flag" src="/flags/{replay.blackCountry}.png" alt="" />
									<span class={blackClass}>
										{formatEloDiff(replay.blackEloDiff)}
									</span>
								</td>
								<td style="width: 20%">
									{formatReplayResult(replay.result)}
								</td>
								<td style="width: 30%">
									{formatPlayedOn(replay.playedOn)}
								</td>
							</tr>
						{/each}
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<div class="color-wrapper" style="width: 530px;">This player hasn't played any games yet.</div>
	{/if}
</div>

<style>
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

	.player-panel {
        width: 600px;
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
</style>
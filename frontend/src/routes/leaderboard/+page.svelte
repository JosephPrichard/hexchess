<script lang="ts">
	import Pagination from '$lib/Pagination.svelte';
	import StatsList from '$lib/components/StatsList.svelte';
	import { UntypedGameModeNameMap, type LeaderboardUser } from '$lib/api/models';
	import Banner from '$lib/Banner.svelte';
	import Dropdown from '$lib/components/Dropdown.svelte';
	import { goto } from '$app/navigation';

	export interface LeaderboardProps {
		page: number;
		pageCount: number;
		userList: LeaderboardUser[];
	}

	const { data: props }: { data: LeaderboardProps } = $props();

	const options = Object.entries(UntypedGameModeNameMap).map(([key, value]) => ({label: value, value: key}));
</script>

<svelte:head>
	<title>Leaderboard - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="title-lg">Leaderboard</div>
	<div class="wrapper">
		<div class="dropdown-wrapper">
			<Dropdown
				options={options}
				selected={"TIMED_1+0"}
				onChange={async (value) => {
				await goto(`/leaderboard?mode=${encodeURIComponent(value)}`);
			}}
			/>
		</div>
		<StatsList userList={props.userList} />
	</div>
</div>
<Pagination targetPage={props.page} totalPages={props.pageCount} />

<style>
	.wrapper {
		width: 90%;
	}

	@media (min-width: 768px) {
		.wrapper {
			width: 700px;
		}
	}

	.dropdown-wrapper {
		margin-bottom: 20px;
		width: 200px;
	}
</style>
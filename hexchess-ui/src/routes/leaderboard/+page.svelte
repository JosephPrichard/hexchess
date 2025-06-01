<script lang="ts">
	import Banner from '$lib/components/Banner.svelte';
	import Pagination from '$lib/components/Pagination.svelte';
	import StatsList from '$lib/components/stats/StatsList.svelte';
	import type { User } from '$lib/models';

	export interface LeaderboardProps {
		page: number;
		pageCount: number;
		userList: User[];
	}

	const { data: props }: { data: LeaderboardProps } = $props();
</script>

<svelte:head>
	<title>Leaderboard - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="title-lg">Leaderboard</div>
	<div class="wrapper">
		<StatsList userList={props.userList} />
	</div>
</div>
<Pagination targetPage={props.page} totalPages={props.pageCount} />

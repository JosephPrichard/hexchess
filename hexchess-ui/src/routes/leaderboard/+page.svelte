<script lang="ts">
	import Banner from '$lib/Banner.svelte';
	import Pagination from '$lib/Pagination.svelte';
	import StatsList from '$lib/components/stats/StatsList.svelte';
	import type { UserModel } from '$lib/api/model';

	export interface LeaderboardProps {
		page: number;
		pageCount: number;
		userList: UserModel[];
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

<script lang="ts">
	import Pagination from '$lib/Pagination.svelte';
	import StatsList from '$lib/components/StatsList.svelte';
	import type { LbdUserModel, UserModel } from '$lib/api/models';
	import Banner from '$lib/Banner.svelte';
	import MagnifyingGlass from "$lib/icons/MagnifyingGlass.svelte";

	export interface SearchProps {
		searchText: string;
		page: number;
		userList: LbdUserModel[];
		message: string;
	}

	const { data: props }: { data: SearchProps } = $props();
</script>

<svelte:head>
	<title>Search - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="title-lg">
		Players Search
	</div>
	<div class="wrapper">
		<form class="search-wrapper">
			<label for="username"></label>
			<input name="username" placeholder="Username" class="username-input" value={props.searchText} />
			<div class="search-icon">
				<MagnifyingGlass/>
			</div>
		</form>
		{#if props.message}
			<div class="color-wrapper">{props.message}</div>
		{:else if props.searchText}
			<StatsList userList={props.userList} />
		{/if}
	</div>
</div>
{#if props.searchText}
	<Pagination targetPage={props.page} />
{/if}

<style>
	.search-wrapper {
		display: flex;
		flex-direction: row;
		margin-right: 10px;
		margin-bottom: 10px;
	}

	.search-icon {
		top: 7px;
		right: 30px;
		position: relative;
	}

	.wrapper {
		width: 90%;
	}

	@media (min-width: 768px) {
		.wrapper {
			width: 700px;
		}
	}

	.username-input {
		width: 90%;
	}

	@media (min-width: 768px) {
		.username-input {
			width: 50%;
		}
	}
</style>
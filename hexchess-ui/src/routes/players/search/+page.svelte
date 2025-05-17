<script lang="ts">
	import Banner from '$lib/components/Banner.svelte';
	import Pagination from '$lib/components/Pagination.svelte';
	import type { UserView } from '$lib/models';
	import StatsList from '$lib/components/StatsList.svelte';

	export interface SearchProps {
		searchText: string;
		page: number;
		userList: UserView[];
	}

	const { data: props }: { data: SearchProps } = $props();
</script>

<svelte:head>
	<title>Search - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="title-lg">Search</div>
	<div class="wrapper">
		<form class="form-wrapper">
			<label for="username"></label>
			<input id="username" name="username" placeholder="Username" style="width: 50%; top: 4px; position: relative" value={props.searchText} />
			<div class="button-submit-vertical-form">
				<button class="button button-grey" type="submit"> Search </button>
			</div>
		</form>
		{#if props.searchText}
			<StatsList userList={props.userList} />
		{/if}
	</div>
</div>
{#if props.searchText}
	<Pagination targetPage={props.page} />
{/if}

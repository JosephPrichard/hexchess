<script lang="ts">
	import Pagination from '$lib/Pagination.svelte';
	import StatsList from '$lib/components/stats/StatsList.svelte';
	import type { UserModel } from '$lib/api/models';
	import Banner from '$lib/Banner.svelte';

	export interface SearchProps {
		searchText: string;
		page: number;
		userList: UserModel[];
		message: string;
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
			<input name="username" placeholder="Username" class="username-input" value={props.searchText} />
			<div class="button-submit-vertical-form">
				<button class="button button-grey" type="submit"> Search </button>
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
	.username-input {
        width: 50%;
		top: 4px;
		position: relative;
	}

    .button-submit-vertical-form {
        position: relative;
        display: inline-block;
        top: 3px;
        margin-left: 10px;
    }
</style>
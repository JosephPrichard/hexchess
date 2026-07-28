<script lang="ts">
	import { getClientSession } from '$lib/utils/storage';
	import type { Session } from './api/models';
	import { onMount } from 'svelte';
	import ProfilePic from '$lib/components/ProfilePic.svelte';
	import ChallengeIcon from "$lib/icons/ChallengeIcon.svelte";
	import SettingsIcon from "$lib/icons/SettingsIcon.svelte";
	import MagnifyingGlass from "$lib/icons/MagnifyingGlass.svelte";
	import {goto} from "$app/navigation";
	import services from "$lib/api/services";
	import {MediaQuery} from "svelte/reactivity";

	const id = $props.id();

	let client: Session | null = $state(null);
	let searchText = $state('');
	let challengesCount = $state(0);

	function initClientSession() {
		client = getClientSession();
	}

	onMount(() => {
		initClientSession();
		window.addEventListener('storage', initClientSession);
		return () => {
			window.removeEventListener('storage', initClientSession);
		};
	});

	onMount(async () => {
		const [data, err] = await services.getChallengesCount();
		if (data) {
			challengesCount = data.count;
		} else {
			logger.error('Failed to get challenges count', err);
		}
	})

	async function handleSearchSubmit(e: Event) {
		e.preventDefault();
		await goto(`/players/search?username=${encodeURIComponent(searchText)}`);
	}

	const isLarge = new MediaQuery('min-width: 1068px');
</script>

<link href="https://fonts.googleapis.com/css2?family=Gidole&display=swap" rel="stylesheet" />
<div class="banner blue-bg" id="banner">
	<a class="banner-elem color-hover text-md" href="/">
		<img alt="" class="logo-symbol" src="/pieces/white-queen.png" />
		<span class="logo-font"> Hexchess </span>
	</a>
	<a class="banner-elem color-hover" href="/">
		Play
	</a>
	<a class="banner-elem color-hover" href="/leaderboard">
		Leaderboard
	</a>
	<a class="banner-elem color-hover" href="/replays">
		Replays
	</a>
	<a class="banner-elem color-hover" href="/tournaments">
		Tournaments
	</a>
	<div class="banner-elem color-hover">
		{#if isLarge.current}
			<form onsubmit={handleSearchSubmit}>
				<input
						id="text"
						type="text"
						class="search-input"
						bind:value={searchText}
						autocomplete="off"
						autocapitalize="off"
						spellcheck="false"
				/>
			</form>
			<div class="search-icon">
				<MagnifyingGlass/>
			</div>
		{:else}
			Search
		{/if}
	</div>
	{#if client}
		<div class="client-panels">
			<a class="banner-elem color-hover challenge-icon-wrapper" href="/challenges" id="challenge-link">
				<ChallengeIcon/>
				{#if challengesCount > 0}
					<div class="challenges-count">
						{challengesCount}
					</div>
				{/if}
			</a>
			<a class="banner-elem color-hover" href="/profile" id="settings-link">
				<SettingsIcon/>
			</a>
			<a aria-label="user-link-{id}" class="banner-elem color-hover" href={`/players/${client.id}`}>
				{client.username}
				<span style="margin-left: 10px"></span>
				<ProfilePic userId={client.id} size={45} unique/>
			</a>
		</div>
	{:else}
		<a aria-label="login-link-{id}" class="banner-elem color-hover" href="/login"> Login </a>
	{/if}
</div>

<style>
	.challenge-icon-wrapper {
		position: relative !important;
	}

	.challenges-count {
		position: absolute;
		text-align: center;
		font-size: 12px;
		top: 5px;
		right: 1px;
		background-color: rgb(183, 55, 78, 0.85);
		color: white;
		border-radius: 50%;
		width: 16px;
		height: 16px;
	}

	.search-input {
		height: 30px;
		display: flex;
		align-items: center;
		padding: 0 30px 0 10px;
	}

	.search-icon {
		right: 15px;
		position: relative;
	}

	.client-panels {
		display: flex;
 		align-items: center;
		margin-left: auto;
	}

    .banner {
        height: 75px;
        margin-bottom: 20px;
        display: flex;
        align-items: center;
        padding-left: 10px;
        padding-right: 10px;
        user-select: none;
        font-weight: 500;

        width: calc(100% - 20px);

        background: rgb(43, 43, 43);
        box-shadow: 0 1px rgb(22, 22, 22);
    }

	@media (max-width: 868px) {
		.banner {
			height: fit-content;
			display: flex;
			flex-direction: column;
			padding-top: 10px;
			padding-bottom: 10px;
		}
	}

    .banner-elem {
        font-size: 17px;
        height: 40%;
        display: flex;
        justify-content: center;
        align-items: center;
        padding: 0 10px;
        cursor: pointer;
        border-radius: 6px;
        user-select: none;
        text-decoration: none;
    }

	@media (max-width: 868px) {
		.banner-elem {
			margin-top: 5px;
			margin-bottom: 5px;
		}
	}

    .logo-font {
        position: relative;
        top: 1px;
        font-family: 'Gidole', sans-serif;
        font-weight: 100;
        font-size: 28px;
    }

    .logo-symbol {
        height: 45px;
        width: auto;
        margin-right: 5px;
        opacity: 0.75;
    }
</style>
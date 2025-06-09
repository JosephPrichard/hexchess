<script lang="ts">
	import { onMount } from 'svelte';
	import { getClientSession } from '$lib/utils/storage';
	import type { SessionModel } from '$lib/api/model';

	const id = $props.id();

	let client: SessionModel | null = $state(null);

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
</script>

<link href="https://fonts.googleapis.com/css2?family=Gidole&display=swap" rel="stylesheet" />
<div class="banner blue-bg" id="banner">
	<a class="banner-elem color-hover text-md" href="/">
		<img alt="" class="logo-symbol" src="/pieces/white-queen.png" />
		<span class="logo-font"> Hexchess </span>
	</a>
	<a class="banner-elem color-hover" href="/"> Play </a>
	<a class="banner-elem color-hover" href="/leaderboard"> Leaderboard </a>
	<a class="banner-elem color-hover" href="/players/search"> Search </a>
	{#if client}
		<a class="banner-elem color-hover" href="/challenges" id="challenge-link"> Challenges </a>
		<a class="banner-elem color-hover" href="/profile" id="settings-link"> Profile </a>
		<a aria-label="user-link-{id}" class="banner-elem color-hover" href={`/players/${client.id}`}>
			{client.username}
		</a>
	{:else}
		<a aria-label="login-link-{id}" class="banner-elem color-hover" href="/login"> Login </a>
	{/if}
</div>

<style>
    .banner {
        height: 75px;
        margin-bottom: 20px;
        display: flex;
        align-items: center;
        padding-left: 15%;
        padding-right: 15%;
        user-select: none;
        font-weight: 500;

        width: 70%;

        background: rgb(43, 43, 43);
        box-shadow: 0 1px rgb(22, 22, 22);
    }

    .banner-elem {
        font-size: 17px;
        height: 40%;
        display: flex;
        justify-content: center;
        align-items: center;
        margin: 5px 5px;
        padding: 15px 10px;
        cursor: pointer;
        border-radius: 6px;
        user-select: none;
        text-decoration: none;
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
<script lang="ts">
    import { onMount } from 'svelte';
    import { getClientSession } from '$lib/local';
    import type { SessionView } from '$lib/models';

    const id = $props.id();

    let client: SessionView | null = $state(null);

    onMount(() => {
        client = getClientSession();
    });
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
    <a class="banner-elem color-hover" href="/players/search"> 
        Search 
    </a>
    {#if client}
        <a class="banner-elem color-hover" href="/challenges" id="challenge-link">
            Challenges
        </a>
        <a class="banner-elem color-hover" href="/profile" id="settings-link">
            Profile
        </a>
        <a aria-label="user-link-{id}" class="banner-elem color-hover" href={`/players/${client.id}`}>
            {client.username}
        </a>
    {:else}
        <a aria-label="login-link-{id}" class="banner-elem color-hover" href="/login">
            Login
        </a>
    {/if}
</div>

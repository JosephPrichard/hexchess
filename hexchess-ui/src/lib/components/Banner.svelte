<script lang="ts">
    import type { Session } from '$lib/models';
    import { onMount } from 'svelte';
    import { getClientSession } from '$lib/utils';

    const id = $props.id();

    let session: Session | undefined = $state(undefined);

    onMount(() => {
        session = getClientSession();
    });
</script>

<link href="https://fonts.googleapis.com/css2?family=Gidole&display=swap" rel="stylesheet" />
<div class="banner blue-bg" id="banner">
    <a class="banner-elem color-hover text-md" href="/static">
        <img alt="" class="logo-symbol" src="%sveltekit.assets%/pieces/white-queen.png" />
        <span class="logo-font"> Hexchess </span>
    </a>
    <a class="banner-elem color-hover" href="/static"> Play </a>
    <a class="banner-elem color-hover" href="/leaderboard"> Leaderboard </a>
    <a class="banner-elem color-hover" href="/players/search"> Search </a>
    <a class="banner-elem color-hover" href="/challenges" id="challenge-link" style:display={session ? '' : 'none'}> Challenges </a>
    <a class="banner-elem color-hover" href="/profile" id="settings-link"> Profile </a>
    <a aria-label="user-link-{id}" class="banner-elem color-hover" id="user-link" style:display={session ? '' : 'none'}></a>
    <a aria-label="login-link-{id}" class="banner-elem color-hover" href="/login" id="login-link" style:display={session ? 'none' : ''}> Login </a>
</div>

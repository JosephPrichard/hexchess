<script lang="ts">
    import Banner from '$lib/components/Banner.svelte';
    import { postLogin, unwrap } from '$lib/api';
    import { goto } from '$app/navigation';
    import { createMessage } from '$lib/response';

    let username = $state('');
    let password = $state('');
    let isLoading = $state(false);

    async function onSubmit(e: MouseEvent) {
        e.preventDefault();

        isLoading = true;
        const { ok, resp, err } = await unwrap(postLogin(username, password));

        if (ok) {
            console.log(resp);
            await goto('/');
        } else {
            const message = createMessage(err);
            console.log(message);
        }

        isLoading = false;
    }
</script>

<svelte:head>
    <title>Login - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
    <div class="title-lg">Login</div>
    <div style="width: 500px">
        <form class="form-wrapper" id="login-form">
            <label for="username-login" style="font-size: 17px">Username</label>
            <input bind:value={username} id="username-login" name="username" placeholder="Username" style="margin-top: 5px; margin-bottom: 10px" />

            <label for="password-login" style="font-size: 17px">Password</label>
            <input bind:value={password} id="password-login" name="password" placeholder="Password" style="margin-top: 5px; margin-bottom: 15px" type="password" />

            {#if isLoading}
                <div class="loader"></div>
            {:else}
                <button id="login-form-submit" class="button button-grey" type="submit" onclick={onSubmit}> Login </button>
            {/if}

            <div style="margin-top: 20px;">
                Don't have an account? Register
                <a href="/register" style="color: cornflowerblue;"> here! </a>
            </div>
        </form>
    </div>
</div>

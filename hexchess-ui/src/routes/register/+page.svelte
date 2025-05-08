<script lang="ts">
    import Banner from '$lib/components/Banner.svelte';
    import { postRegister } from '$lib/api';
    import { goto } from '$app/navigation';
    import { createMessage } from '$lib/response';

    let username = $state('');
    let password = $state('');
    let confirmPassword = $state('');
    let isLoading = $state(false);

    async function onSubmit(e: MouseEvent) {
        e.preventDefault();

        isLoading = true;
        const { ok, resp, err } = await postRegister(username, password, confirmPassword);

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
    <title>Register - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
    <div class="title-lg">Register</div>
    <div style="width: 500px">
        <form class="form-wrapper" id="register-form">
            <label for="username-register" style="font-size: 17px">Username</label>
            <input bind:value={username} id="username-register" name="username" placeholder="Username" style="margin-top: 5px; margin-bottom: 10px;" />

            <label for="password-register" style="font-size: 17px">Password</label>
            <input bind:value={password} id="password-register" name="password" placeholder="Password" style="margin-top: 5px; margin-bottom: 10px;" type="password" />

            <label for="password-retype-register" style="font-size: 17px">Confirm Password</label>
            <input bind:value={confirmPassword} id="password-retype-register" name="password" placeholder="Password" style="margin-top: 5px; margin-bottom: 15px;" type="password" />

            {#if isLoading}
                <div class="loader"></div>
            {:else}
                <button id="register-form-submit" class="button button-grey" type="submit" onclick={onSubmit}> Register </button>
            {/if}
        </form>
    </div>
</div>

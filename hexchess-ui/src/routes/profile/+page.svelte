<script lang="ts">
    import type { UserView } from '$lib/models';
    import { getCountries, postLogout, postUpdatePassword, postUpdateUser } from '$lib/api';
    import { goto } from '$app/navigation';
    import { onMount } from 'svelte';

    export interface ProfileProps  {
        user: UserView;
    }

    const { data }: { data: ProfileProps } = $props();
    const { user } = data;

    let showCountryOptions = $state(false);
    let countryList: string[] = $state([]);
    let username = $state(user.username);
    let bio = $state(user.bio);
    let country = $state(user.country);
    let password = $state('');
    let newPassword = $state('');
    let retypePassword = $state('');

    onMount(async () => {
        const { ok, status, resp, err } = await getCountries();
        if (!ok) {
            console.error(ok, status, resp, err );
        }
        countryList = resp || [];
    });

    async function onSubmitUser(e: MouseEvent) {
        e.preventDefault();

        const { ok, status, resp, err } = await postUpdateUser(username, bio, country);
        console.error(ok, status, resp, err );
    }

    async function onSubmitPassword(e: MouseEvent) {
        e.preventDefault();

        const { ok, status, resp, err } = await postUpdatePassword(password, newPassword, retypePassword);
        console.error(ok, status, resp, err );
    }

    async function onSelectCountry(country: string) {
        const { ok, status, resp, err } = await postUpdateUser(username, bio, country);
        console.error(ok, status, resp, err );
    }

    async function onClickSignOut() {
        const { ok, status, resp, err } = await postLogout();
        if (ok) {
            await goto('/');
        } else {
            console.error(ok, status, resp, err );
        }
    }
</script>

<svelte:head>
    <title>Profile - Hexchess</title>
</svelte:head>
<div class="center-horizontal-container">
    <div style="width: 500px">
        <div class="title-lg" style="padding-left: 0">Edit Profile</div>
        <form class="form-wrapper">
            <label for="username" style="font-size: 17px">Username</label>
            <input bind:value={username} name="username" placeholder="Username" style="margin: 5px 0 10px;" />

            <label for="bio" style="font-size: 17px">Biography</label>
            <textarea bind:value={bio} name="bio" rows="8" style="padding: 8px; font-size: 14px"></textarea>

            <label for="country" style="font-size: 17px">Country</label>
            <div class="country-wrapper">
                <button class="invisible-button" onclick={() => (showCountryOptions = false)}>
                    <img alt={country} class="country-image" src={`%sveltekit.assets%/flags/${country}.png`} />
                </button>
                {#if showCountryOptions}
                    <div class="country-picker">
                        {#each countryList as country (country)}
                            <button class="invisible-button" onclick={() => onSelectCountry(country)}>
                                <img class="country-image" src={`%sveltekit.assets%/flags/${country}.png`} alt={country} />
                            </button>
                        {/each}
                    </div>
                {/if}
            </div>

            <button class="button button-grey" onclick={onSubmitUser} style="margin-top: 15px;" type="submit"> Submit </button>
        </form>

        <div class="title-lg" style="padding-left: 0">Update Password</div>
        <form class="form-wrapper">
            <label for="password" style="font-size: 17px"> Current Password </label>
            <input bind:value={password} name="password" placeholder="Password" style="margin: 5px 0 10px;" type="password" />

            <label for="new-password" style="font-size: 17px"> New Password </label>
            <input bind:value={newPassword} name="new-password" placeholder="New Password" style="margin: 5px 0 15px;" type="password" />

            <label for="retype-password" style="font-size: 17px"> Retype New Password </label>
            <input bind:value={retypePassword} name="retype-password" placeholder="Retype Password" style="margin: 5px 0 15px;" type="password" />

            <button class="button button-grey" onclick={onSubmitPassword} type="submit"> Submit </button>
            <div id="update-password-form-message" style="margin-top: 20px; min-height: 20px;"></div>
        </form>

        <br />
        <button class="button button-red" onclick={onClickSignOut}> Sign Out </button>
        <div style="height: 100px;"></div>
    </div>
</div>

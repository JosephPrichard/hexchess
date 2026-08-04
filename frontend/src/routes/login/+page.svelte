<script lang="ts">
	import { services } from '$lib/api/services';
	import { goto } from '$app/navigation';
	import { setClientSession } from '$lib/utils/storage';
	import { fade } from 'svelte/transition';
	import { onMount } from 'svelte';
	import { env } from '$env/dynamic/public';
	import type { ServiceResponse, Session } from '$lib/api/models';
	import Banner from '$lib/Banner.svelte';
	import { handleErrorAsArray } from '$lib/utils/error';

	let username = $state('');
	let password = $state('');
	let messages = $state<string[]>([]);
	let isLoading = $state(false);
	let removeMessage: ReturnType<typeof setTimeout> | undefined = undefined;

	async function onLoginComplete([data, err]: [data: Session | undefined, err: ServiceResponse | undefined]) {
		if (data) {
			setClientSession(data);
			await goto('/');
		} else {
			if (removeMessage !== undefined) {
				clearTimeout(removeMessage);
			}
			messages = handleErrorAsArray(err);
			removeMessage = setTimeout(() => messages = [], 5000);
		}
	}

	async function onSubmit(e: MouseEvent) {
		e.preventDefault();
		isLoading = true;
		await onLoginComplete(await services.postLogin(username, password));
		isLoading = false;
	}

	const clientId = env.PUBLIC_APP_GOOGLE_CLIENT_ID;

	onMount(() => {
		const google = (window as any).google as any;

		if (clientId === undefined) return;
		google.accounts.id.initialize({
			client_id: clientId,
			callback: async (response: any) => {
				await onLoginComplete(await services.postGoogleLogin(response.credential));
			}
		});
		const googleBtn = document.getElementById("googleBtn");
		if (googleBtn == null) {
			return;
		}
		google.accounts.id.renderButton(googleBtn, {
			theme: "filled_blue",
			size: "large",
			type: "standard"
		});
	});
</script>

<svelte:head>
	<title>Login - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="title-lg">Login</div>
	<div class="login-wrapper">
		<form class="form-wrapper">
			<label for="username-login" class="login-font">Username</label>
			<input bind:value={username} class="login-input" name="username" placeholder="Username" />

			<label for="password-login" class="login-font">Password</label>
			<input
				bind:value={password}
				class="login-input"
				name="password"
				placeholder="Password"
				type="password"
			/>

			<button class="button button-grey" type="submit" onclick={onSubmit}>
				{#if isLoading}
					<div class="loader"></div>
				{:else}
					Login
				{/if}
			</button>

			<div class:show-gbutton={clientId} class:hide-gbutton={!clientId}>
				<div class="google-button-title divider">
					Or
				</div>
				<div id="googleBtn" class="google-button"></div>
			</div>

			<div style="margin-top: 20px;">
				Don't have an account? Register
				<a href="/register" style="color: cornflowerblue;"> here! </a>
			</div>

			<div class="error-container">
				{#each messages as message}
					<div class="error-box" transition:fade>{message}</div>
				{/each}
			</div>
		</form>
	</div>
</div>

<style>
	.show-gbutton {
		display: block;
	}
	
	.hide-gbutton {
		display: none;
	}

    .google-button-title {
		margin-top: 20px;
		margin-bottom: 20px;
		text-align: center;
	}

	.google-button {
		margin-top: 10px;
		margin-bottom: 20px;
	}

	.login-wrapper {
		width: 500px;
	}

	.login-font {
        font-size: 17px;
	}

	.login-input {
        margin-top: 5px;
		margin-bottom: 10px
	}
</style>
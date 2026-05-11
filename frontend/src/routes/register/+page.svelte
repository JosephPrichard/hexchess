<script lang="ts">
	import { goto } from '$app/navigation';
	import { errorToArray } from '$lib/utils/error';
	import { fade } from 'svelte/transition';
	import { setClientSession } from '$lib/utils/storage';
	import { getNotificationsContext } from '$lib/utils/context';
	import services from '$lib/api/services';
	import Banner from '$lib/Banner.svelte';

	let username = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let isLoading = $state(false);
	let messages = $state<string[]>([]);
	let removeMessage: ReturnType<typeof setTimeout> | undefined = undefined;

	const { addNotification } = getNotificationsContext();

	async function onSubmit(e: MouseEvent) {
		e.preventDefault();

		isLoading = true;
		const [data, err] = await services.postRegister(username, password, confirmPassword);

		if (data) {
			setClientSession(data);

			const message = 'Registration was successful!';
			addNotification({ type: 'string', message, isSuccess: true });

			await goto('/');
		} else {
			if (removeMessage !== undefined) {
				clearTimeout(removeMessage);
			}
			messages = errorToArray(err);
			removeMessage = setTimeout(() => messages = [], 5000);
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
	<div class="register-wrapper">
		<form class="form-wrapper">
			<label for="username-register" style="font-size: 17px">Username</label>
			<input
				bind:value={username}
				id="username-register"
				name="username"
				placeholder="Username"
				style="margin-top: 5px; margin-bottom: 10px;"
			/>

			<label for="password-register" style="font-size: 17px">Password</label>
			<input
				bind:value={password}
				id="password-register"
				name="password"
				placeholder="Password"
				style="margin-top: 5px; margin-bottom: 10px;"
				type="password"
			/>

			<label for="password-retype-register" style="font-size: 17px">Confirm Password</label>
			<input
				bind:value={confirmPassword}
				id="password-retype-register"
				name="password"
				placeholder="Password"
				style="margin-top: 5px; margin-bottom: 15px;"
				type="password"
			/>

			<button class="button button-grey" type="submit" onclick={onSubmit}>
				{#if isLoading}
					<div class="loader"></div>
				{:else}
					Register
				{/if}
			</button>

			<div class="error-container">
				{#each messages as message}
					<div class="error-box" transition:fade>{message}</div>
				{/each}
			</div>
		</form>
	</div>
</div>

<style>
	.register-wrapper {
        width: 500px;
	}
</style>

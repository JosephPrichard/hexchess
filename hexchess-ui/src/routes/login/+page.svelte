<script lang="ts">
	import Banner from '$lib/Banner.svelte';
	import services from '$lib/api/services';
	import { goto } from '$app/navigation';
	import { makeMessage } from '$lib/utils/error';
	import { setClientSession } from '$lib/utils/storage';
	import { getNotificationsContext } from '$lib/utils/context';

	let username = $state('');
	let password = $state('');
	let isLoading = $state(false);

	const { addNotification } = getNotificationsContext();

	async function onSubmit(e: MouseEvent) {
		e.preventDefault();

		isLoading = true;
		const [data, err] = await services.postLogin(username, password);

		if (data) {
			console.log('Logged in', data);
			setClientSession(data);
			await goto('/');
		} else {
			const message = makeMessage(err);
			console.log(message);
			addNotification({ type: 'string', message, isSuccess: false });
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

			<div style="margin-top: 20px;">
				Don't have an account? Register
				<a href="/register" style="color: cornflowerblue;"> here! </a>
			</div>
		</form>
	</div>
</div>

<style>
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
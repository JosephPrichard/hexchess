<script lang="ts">
	import { clearClientSession, updateClientSession as updateClientUser } from '$lib/utils/storage';
	import { getNotificationsContext } from '$lib/utils/context';
	import type { UserModel } from '$lib/api/models';
	import services, { baseURL } from '$lib/api/services';
	import Banner from '$lib/Banner.svelte';
	import ProfilePic from '$lib/components/user/ProfilePic.svelte';
	import { goto } from '$app/navigation';

	export interface ProfileProps {
		countryList: string[];
		user: UserModel;
	}

	const { data: props }: { data: ProfileProps } = $props();

	const { addErrorNotification } = getNotificationsContext();

	let showCountryOptions = $state(false);
	let isLoading = $state(false);
	let username = $state(props.user.username);
	let bio = $state(props.user.bio);
	let country = $state(props.user.country);
	let password = $state('');
	let newPassword = $state('');
	let retypePassword = $state('');
	let profilePic: File | undefined;

	async function onSubmitUser(e: MouseEvent) {
		e.preventDefault();
		isLoading = true;

		const [[userData, userErr], profileResp] = await Promise.all([
			services.postUpdateUser(username, bio, country),
			profilePic ? services.postProfilePic(profilePic) : Promise.resolve(undefined)
		]);
		if (userData) {
			updateClientUser(userData);
		} else {
			addErrorNotification(userErr);
		}
		if (profileResp) {
			const [_, profileErr] = profileResp;
			if (profileErr) {
				addErrorNotification(profileErr);
			}
		}

		isLoading = false;
	}

	async function onSubmitPassword(e: MouseEvent) {
		e.preventDefault();

		const [_, err] = await services.postUpdatePassword(password, newPassword, retypePassword);
		if (err) {
			addErrorNotification(err);
		}
	}

	async function onSelectCountry(e: MouseEvent, newCountry: string) {
		e.preventDefault();
		country = newCountry;
		showCountryOptions = false;
	}

	async function onClickSignOut() {
		const [data, err] = await services.postLogout();
		if (data) {
			clearClientSession();
			await goto("/");
		} else if (err) {
			addErrorNotification(err);
		}
	}

	function toggleCountryDropdown(e: MouseEvent) {
		e.preventDefault();
		showCountryOptions = !showCountryOptions;
	}

	async function onFileInputChange(event: Event) {
		const input = event.target as HTMLInputElement;
		if (!input.files) return;

		const file = Array.from(input.files)[0];
		if (!file) return;

		profilePic = file;

		// updates every instance of the profile pic for this user with this image, to keep in sync
		const profilePicElements = document.getElementsByClassName('profile-pic-'+props.user.id);
		for (const pic of profilePicElements) {
			const picElem = pic as HTMLImageElement;
			const imageUrl = URL.createObjectURL(file);
			picElem.src = imageUrl;
			picElem.onload = () => URL.revokeObjectURL(imageUrl);
		}
	}
</script>

<svelte:head>
	<title>Profile - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="profile-wrapper">
		<div class="title-lg" style="padding-left: 0">Edit Profile</div>
		<form class="form-wrapper">
			<div class="file-wrapper">
				<input type="file" id="file-input" class="file-input" accept="image/*" onchange={onFileInputChange}/>
				<label for="file-input" class="file-upload-label">
					<ProfilePic userId={props.user.id} size={125}/>
				</label>
			</div>

			<label for="username" style="font-size: 17px">Username</label>
			<input bind:value={username} id="username" placeholder="Username" autocomplete="off" style="margin: 5px 0 10px;" />

			<label for="bio" style="font-size: 17px">Biography</label>
			<textarea bind:value={bio} id="bio" rows="8" style="padding: 8px; font-size: 14px"></textarea>

			<span style="font-size: 17px">Country</span>
			<div class="country-wrapper">
				<button class="invisible-button" onclick={toggleCountryDropdown}>
					<img alt={country} class="country-image" src="/flags/{country}.png" />
				</button>
				{#if showCountryOptions}
					<div class="country-picker">
						{#each props.countryList as country, i (i)}
							<button class="invisible-button" onclick={(e) => onSelectCountry(e, country)}>
								<img class="country-image" src="/flags/{country}.png" alt={country} />
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<button class="button button-grey" onclick={onSubmitUser} style="margin-top: 15px;" type="submit">
				{#if isLoading}
					<div class="loader"></div>
				{:else}
					Save
				{/if}
			</button>
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


<style>
    .country-wrapper {
        margin-top: 10px;
        margin-bottom: 10px;
    }

    .country-image {
        width: 80px;
        height: 55px;
        border-radius: 5px;
        cursor: pointer;
        display: inline-block;
    }

    .country-picker {
        position: absolute;
        z-index: 2;
        overflow-y: scroll;
        background-color: rgb(50, 50, 50);
        box-shadow: 0 0 0 1px rgb(40, 40, 40);
        border-radius: 10px;
        padding: 25px;
        width: calc(87px * 5);
        height: calc(500px);
    }

	.profile-wrapper {
        width: 500px;
	}

	.file-input {
        display: none;
    }

    .file-upload-label {
        cursor: pointer;
    }

	.file-wrapper {
		margin-bottom: 25px;
	}
</style>
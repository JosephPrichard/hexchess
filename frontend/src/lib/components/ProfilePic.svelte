<script lang="ts">
	import { backendBaseURL } from '$lib/api/services.js';

	export interface ProfilePicProps {
		userId: number;
		size: number;
		unique?: boolean;
	}

	const id = $props.id();

	const props: ProfilePicProps = $props();
</script>

<img
	style="width: {props.size}px; height: {props.size}px;"
	src={backendBaseURL() + "/users/profile-pics?userId=" + props.userId}
	alt=""
	class="pfp-icon profile-pic-{props.userId}{props.unique ? id : ''}"
/>

<style>
    .pfp-icon {
        border-radius: 5px;
        background-color: rgb(62, 62, 62);
    }
</style>
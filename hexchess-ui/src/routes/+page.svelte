<script lang="ts">
	import Banner from '$lib/components/Banner.svelte';
	import CreateGame from '$lib/components/modals/CreateGame.svelte';
	import type { TimeControl } from '$lib/models.js';
	import type { ColorSelect } from '$lib/models.js';
	import { postCreateGame, unwrap } from '$lib/api';
	import { createMessage } from '$lib/error';
	import { getNotificationsContext } from '$lib/context';
	import { goto } from '$app/navigation';

	let showCreateModal = $state(false);

	const { addNotification } = getNotificationsContext();

	async function onSubmitCreateGame(timeControl: TimeControl, color: ColorSelect) {
		const { ok, resp, err } = await unwrap(postCreateGame(timeControl, color));
		showCreateModal = false;
		if (ok || resp) {
			await goto(`play?id=${resp}`);
		} else {
			const message = createMessage(err);
			addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
		}
	}
</script>

<svelte:head>
	<title>Hexchess</title>
</svelte:head>
<Banner />
<CreateGame title="Create a Game?" show={showCreateModal} onSubmit={onSubmitCreateGame} onClose={() => (showCreateModal = false)} />
<div class="center-horizontal-container">
	<div class="center-vertical-container">
		<div>
			<button class="button button-grey" id="challenge-button" onclick={() => (showCreateModal = true)}>
				Play a Friend
			</button>
			<button class="button button-grey" id="challenge-button">
				Find a Match
			</button>
		</div>
	</div>
</div>
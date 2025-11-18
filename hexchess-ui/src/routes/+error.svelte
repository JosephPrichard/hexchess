<script lang="ts">
	import { page } from '$app/state';
	import Error from '$lib/Error.svelte';
</script>

<svelte:head>
	<title>Error {page.status} - Hexchess</title>
</svelte:head>
<Error status={page.status} message={page.error?.message} />

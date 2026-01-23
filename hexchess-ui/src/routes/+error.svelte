<script lang="ts">
	import { page } from '$app/state';
	import Error from '$lib/Error.svelte';
	import Banner from '$lib/Banner.svelte';
</script>

<svelte:head>
	<title>Error {page.status} - Hexchess</title>
</svelte:head>
<Banner/>
<Error status={page.status} message={page.error?.message} />

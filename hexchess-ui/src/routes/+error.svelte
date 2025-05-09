<script lang="ts">
    import Banner from '$lib/components/Banner.svelte';
    import { page } from '$app/state';
    import Error from '$lib/components/Error.svelte';
</script>

<svelte:head>
    <title>Error {page.status} - Hexchess</title>
</svelte:head>
<Banner />
<Error status={page.status} message={page.error?.message} />